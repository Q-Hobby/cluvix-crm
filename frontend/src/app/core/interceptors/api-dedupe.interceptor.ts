import { HttpContextToken, HttpEvent, HttpInterceptorFn, HttpResponse } from '@angular/common/http';
import { finalize, Observable, of, shareReplay, tap } from 'rxjs';

/** Context flags */
export const DEDUPE_DISABLE = new HttpContextToken<boolean>(() => false);
export const DEDUPE_TTL_MS = new HttpContextToken<number>(() => 5000); // 5s
export const DEDUPE_KEY_SUFFIX = new HttpContextToken<string>(() => ''); // key phụ nếu cần
export const DEDUPE_STALE_WHILE_REVALIDATE = new HttpContextToken<boolean>(() => true); // trả stale ngay, refresh nền

/** Domain whitelist — self-host: thêm domain FE production của bạn vào đây. */
const API_BASE_DOMAINS = ['example.com', 'tm', 'localhost'];
const hostMatches = (host: string, base: string) => host === base || host.endsWith('.' + base);
const toURL = (urlStr: string) =>
  new URL(urlStr, (globalThis as any).location?.origin ?? 'http://localhost');

function isApiUrl(urlStr: string): boolean {
  try {
    const u = toURL(urlStr);
    const path = u.pathname;
    if (!path.startsWith('/api/')) return false;
    // Same-origin (relative URL) always allowed
    if (u.origin === (globalThis as any).location?.origin) return true;
    // Cross-origin: check whitelist
    return API_BASE_DOMAINS.some((base) => hostMatches(u.hostname, base));
  } catch {
    return false;
  }
}

function isBinary(respType: string | undefined): boolean {
  const t = (respType || 'json').toLowerCase();
  return t === 'blob' || t === 'arraybuffer';
}

function isLongJobUrl(urlStr: string): boolean {
  try {
    const u = toURL(urlStr);
    if (u.searchParams.get('_export') === '1') return true; // export
    if (u.pathname.includes('/import')) return true; // import
    return false;
  } catch {
    return false;
  }
}

/** ===== In-tab RAM ===== */
const inflight = new Map<string, Observable<HttpEvent<any>>>();
const mem = new Map<string, { ts: number; obs$: Observable<HttpEvent<any>> }>();
const MAX = 500;

/** ===== Write revision ===== */
const WREV_KEY = 'dedupe_wrev'; // global write revision

let wrevMem = 0;
const getWrev = () => {
  try {
    return Number(sessionStorage.getItem(WREV_KEY)) || wrevMem || 0;
  } catch {
    return wrevMem || 0;
  }
};
const bumpWrev = () => {
  try {
    const next = getWrev() + 1;
    sessionStorage.setItem(WREV_KEY, String(next));
    wrevMem = next;
  } catch {
    wrevMem = getWrev() + 1;
  }
};

/** Extract user ID from JWT token */
function extractUserId(authHeader: string | null): string {
  if (!authHeader) return '';
  try {
    const token = authHeader.replace(/^Bearer\s+/i, '');
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.sub || payload.id || payload.user_id || '';
  } catch {
    return '';
  }
}

/** === Key builder ổn định + wrev === */
function sortSearchParams(params: URLSearchParams): string {
  const arr: [string, string][] = [];
  params.forEach((v, k) => arr.push([k, v]));
  arr.sort((a, b) =>
    a[0] === b[0] ? String(a[1]).localeCompare(String(b[1])) : a[0].localeCompare(b[0]),
  );
  return new URLSearchParams(arr).toString();
}

function buildKey(req: any, wrev: number): { key: string; urlStr: string } {
  const u = toURL(req.urlWithParams);
  u.searchParams.delete('__wrev'); // bỏ param kỹ thuật
  if (u.search && u.search.length > 1) u.search = sortSearchParams(u.searchParams);

  const userId = extractUserId(req.headers.get('Authorization'));
  const authMark = userId ? `:u${userId}` : '';
  const urlStr = u.toString();
  const key = `${urlStr}${authMark}:wrev=${wrev}`;
  return { key, urlStr };
}

/** Gắn __wrev vào URL thật để bẻ cache ngoài SW/HTTP */
function withWrev(req: any, wrev: number) {
  return req.clone({ setParams: { __wrev: String(wrev) } });
}

/** ===== Interceptor ===== */
export const apiDedupeInterceptor: HttpInterceptorFn = (req, next) => {
  const isApi = isApiUrl(req.urlWithParams);

  // 0) WRITE: bất kỳ POST/PUT/PATCH/DELETE vào /api/* → bump wrev sau khi thành công
  if (req.method !== 'GET' && isApi) {
    return next(req).pipe(
      tap((ev) => {
        if (ev instanceof HttpResponse && ev.ok) bumpWrev();
      }),
    );
  }

  // Skip các trường hợp không áp dụng cho GET
  if (
    req.method !== 'GET' ||
    req.context.get(DEDUPE_DISABLE) ||
    !isApi ||
    isLongJobUrl(req.urlWithParams) ||
    isBinary(req.responseType as string)
  ) {
    return next(req);
  }

  const ttl = req.context.get(DEDUPE_TTL_MS);
  const wrev = getWrev();
  const suf = req.context.get(DEDUPE_KEY_SUFFIX) || '';
  const swr = req.context.get(DEDUPE_STALE_WHILE_REVALIDATE);

  // Clone req với __wrev để đảm bảo fresh đối với SW/CDN/HTTP cache
  req = withWrev(req, wrev);

  const { key, urlStr } = buildKey(req, wrev);
  const fullKey = key + suf;

  /** 1) RAM micro-cache (5s) */
  const cached = mem.get(fullKey);
  if (cached && Date.now() - cached.ts < ttl) return cached.obs$;

  /** 2) Join inflight (dedupe cùng lúc) */
  const inflight$ = inflight.get(fullKey);
  if (inflight$) return inflight$;

  /** 3) New request */
  const obs$ = next(req).pipe(
    tap((ev) => {
      if (ev instanceof HttpResponse) {
        if (ev.ok) {
          mem.set(fullKey, { ts: Date.now(), obs$: obs$ });
          // KHÔNG writePersisted để luôn fetch mới
          while (mem.size > MAX) trimOldest();
        } else if (ev.status === 404) {
          mem.set(fullKey, { ts: Date.now() - ttl + 10000, obs$: of(ev) });
        }
      }
    }),
    finalize(() => inflight.delete(fullKey)),
    shareReplay({ bufferSize: 1, refCount: false }),
  );

  inflight.set(fullKey, obs$);
  return obs$;
};

function trimOldest() {
  let oldestKey: string | undefined,
    oldestTs = Infinity;
  for (const [k, v] of mem)
    if (v.ts < oldestTs) {
      oldestTs = v.ts;
      oldestKey = k;
    }
  if (oldestKey) mem.delete(oldestKey);
}
