import { DASHBOARD_LAYOUT_VERSION, type DashboardLayoutConfig } from './layout.util';

/**
 * Bản TẠM bố cục Dashboard CRM ở localStorage (PRD FR18, story-09): MỘT khoá `crm_dashboard_layout` chứa
 * `{ "<uid>": CachedLayout }` (CRM: 1 DB = 1 thương hiệu ⇒ khoá theo user, không kèm công ty như ERP) — khoá cố định để nằm được trong `LOGOUT_KEYS` (xoá khi đăng xuất).
 * CHỈ bố cục, KHÔNG số liệu. Đọc hỏng/rác ⇒ coi như không có bản tạm. Lỗi ghi do `StorageService` nuốt.
 */
export interface CachedLayout {
  config: DashboardLayoutConfig | null;
  source: 'user' | 'brand' | 'system';
  updatedAt: number;
}

export interface LayoutStorage {
  load(): unknown;
  save(value: Record<string, CachedLayout>): void;
}

export function ownerKey(uid: number | null | undefined): string | null {
  return uid ? String(uid) : null;
}

function isCached(v: unknown): v is CachedLayout {
  const c = v as CachedLayout;
  if (!c || typeof c !== 'object') return false;
  if (!['user', 'brand', 'system'].includes(c.source)) return false;
  if (c.config === null) return c.source === 'system';
  return !!c.config && c.config.version === DASHBOARD_LAYOUT_VERSION && Array.isArray(c.config.items);
}

export function readCachedLayout(storage: LayoutStorage, owner: string | null): CachedLayout | null {
  if (!owner) return null;
  try {
    const all = storage.load();
    if (!all || typeof all !== 'object' || Array.isArray(all)) return null;
    const v = (all as Record<string, unknown>)[owner];
    return isCached(v) ? v : null;
  } catch {
    return null;
  }
}

/** Ghi bản tạm của 1 chủ; giữ bản của chủ khác (cùng máy nhiều tài khoản). Dữ liệu cũ rác ⇒ bỏ. */
export function writeCachedLayout(storage: LayoutStorage, owner: string | null, value: CachedLayout): void {
  if (!owner) return;
  try {
    const raw = storage.load();
    const all = raw && typeof raw === 'object' && !Array.isArray(raw) ? { ...(raw as Record<string, CachedLayout>) } : {};
    for (const k of Object.keys(all)) if (!isCached(all[k])) delete all[k];
    all[owner] = value;
    storage.save(all);
  } catch {
    /* localStorage bị chặn/đầy ⇒ chạy như không có bản tạm */
  }
}
