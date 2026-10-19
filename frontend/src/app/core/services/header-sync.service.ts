import { inject, Service} from '@angular/core';
import { HttpClient, HttpContext, HttpXhrBackend } from '@angular/common/http';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { DEDUPE_DISABLE } from '@/core/interceptors/api-dedupe.interceptor';
import { environment } from '@env/environment';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { BaseIndexedDBService } from '@/indexeddb/config/service/base-indexed-db.service';
import { TokenService } from '@/core/services/auth/token.service';

@Service()
export class HeaderSyncService {
  private http = inject(HttpClient);
  // Raw HTTP client bypass interceptors — dùng cho startup sync trong APP_INITIALIZER
  private rawHttp = new HttpClient(inject(HttpXhrBackend));
  private tokenService = inject(TokenService);
  private router = inject(Router);
  private registry = inject(MasterDataRegistryService);
  private indexedDB = inject(BaseIndexedDBService);
  private syncPromise: Promise<void> | null = null;
  // Per-table versions (Phương án B) — in-memory cache, lazy-loaded từ IndexedDB
  private tableVersions: Record<string, number> | null = null;
  private tableVersionsLoadPromise: Promise<void> | null = null;
  // Dedup concurrent sync per table — tránh nhiều response cùng trigger sync 1 table
  private tableSyncPromises = new Map<string, Promise<void>>();

  // Fixed key cross-tab lock cho full sync. TTL 30s để tránh stale lock từ tab crash.
  private readonly FULL_SYNC_LOCK_KEY = 'sync-lock-full';

  // Tenant-defining tables — khi đổi cần reload page để refresh toàn bộ UI
  // (switch-branch dropdown, sidebar logo, scope filter, ...). Cùng pattern với
  // switch-branch.ts đã làm khi đổi tenant.
  // NOTE: users KHÔNG nằm đây. Sửa user khác không cần reload tab của staff khác —
  // chỉ refresh users cache. Reload self-user edit handled trong user-form.ts.
  // `business_profile` (crm-standalone story-43, thay `companies` đã bỏ ở story-62): KHÔNG phải bảng
  // IndexedDB — BE `config/business_profile` bump tên này để mọi tab nạp lại hồ sơ (timezone, tiền
  // tệ, logo) qua `refreshBusinessProfile()` lúc khởi động.
  private readonly RELOAD_ON_CHANGE_TABLES: Record<string, string> = {
    business_profile: 'Đã cập nhật thông tin thương hiệu',
    branches: 'Đã cập nhật thông tin chi nhánh',
  };

  private acquireFullSyncLock(): boolean {
    const existing = localStorage.getItem(this.FULL_SYNC_LOCK_KEY);
    if (existing && Date.now() - Number(existing) < 30_000) return false;
    localStorage.setItem(this.FULL_SYNC_LOCK_KEY, Date.now().toString());
    return true;
  }

  private releaseFullSyncLock(): void {
    localStorage.removeItem(this.FULL_SYNC_LOCK_KEY);
  }

  /**
   * Gọi từ APP_INITIALIZER: nếu IndexedDB rỗng (version=0) thì sync ngay trước khi
   * component nào load. Tránh race condition: component đọc DB trống → render empty.
   *
   * Dùng rawHttp (bypass interceptors) + token thủ công + timeout 8s để tránh hang
   * trong APP_INITIALIZER phase khi interceptors chưa ready.
   */
  async ensureSyncedIfEmpty(): Promise<void> {
    const localVersion = await this.indexedDB.getLocalVersion().catch(() => 0);
    if (localVersion !== 0) return;

    const timeout = new Promise<never>((_, reject) =>
      setTimeout(() => reject(new Error('startup sync timeout')), 10_000),
    );

    try {
      await Promise.race([this.doInitialSync(), timeout]);
    } catch {
      // Silently fail — header-based sync sẽ xử lý ở API call đầu tiên
    }
  }

  private async doInitialSync(): Promise<void> {
    const token = this.tokenService.getAccessToken();
    const headers: Record<string, string> = token
      ? { Authorization: `Bearer ${token}` }
      : {};

    const httpRes = await firstValueFrom(
      this.rawHttp.get<any>(
        `${environment.apiUrl}/crm/indexeddb/sync`,
        { headers, withCredentials: true, observe: 'response' },
      ),
    );

    await this.registry.syncAllTables(httpRes.body);

    await this.indexedDB.setLocalVersion(Math.floor(Date.now() / 1000));

    const tablesHeader = httpRes.headers.get('X-Dexie-Versions');
    if (tablesHeader) {
      const parsed = this.parseTableVersionsHeader(tablesHeader);
      this.tableVersions = parsed;
      await this.indexedDB.setTableVersions(parsed).catch(() => {});
    }
  }

  async checkVersionFromHeaders(headers: any): Promise<void> {
    try {
      // Phương án B: per-table version compare qua header X-Dexie-Versions
      // Format: "companies=1745,branches=1746,__all__=1750"
      // Sentinel "__all__" → handler legacy bump → trigger full sync
      // Empty/missing → fresh BE chưa bump, hoặc fallback
      const tableVersionsHeader = headers.get('X-Dexie-Versions') ?? '';
      await this.handleTableVersionsHeader(tableVersionsHeader, headers);
    } catch {
      // Không throw để không block request
    }
  }

  private isEmrRoute(): boolean {
    return this.router.url.startsWith('/emr') || this.router.url.startsWith('/config/emr');
  }

  /** Parse "companies=1745,branches=1746,units=1750" → Record<string, number> */
  private parseTableVersionsHeader(value: string): Record<string, number> {
    const result: Record<string, number> = {};
    if (!value) return result;
    for (const part of value.split(',')) {
      const eq = part.indexOf('=');
      if (eq <= 0) continue;
      const name = part.slice(0, eq).trim();
      const ts = Number(part.slice(eq + 1));
      if (name && !isNaN(ts)) result[name] = ts;
    }
    return result;
  }

  private async ensureTableVersionsLoaded(): Promise<void> {
    if (this.tableVersions !== null) return;
    if (!this.tableVersionsLoadPromise) {
      this.tableVersionsLoadPromise = this.indexedDB
        .getTableVersions()
        .then((v) => {
          this.tableVersions = v ?? {};
        })
        .catch(() => {
          this.tableVersions = {};
        });
    }
    await this.tableVersionsLoadPromise;
  }

  /** Sync 1 table với dedup — nhiều response cùng trigger thì share 1 promise */
  private syncTableOnce(tableName: string): Promise<void> {
    const existing = this.tableSyncPromises.get(tableName);
    if (existing) return existing;
    const p = this.registry
      .invalidateAndSync(tableName)
      .catch((err) => {
        console.warn(`Table sync failed for ${tableName}:`, err);
      })
      .finally(() => {
        this.tableSyncPromises.delete(tableName);
      });
    this.tableSyncPromises.set(tableName, p);
    return p;
  }

  private async handleTableVersionsHeader(headerValue: string, headers?: any): Promise<void> {
    const serverVersions = this.parseTableVersionsHeader(headerValue);
    const localGlobalVersion = await this.indexedDB.getLocalVersion().catch(() => 0);

    // Fresh install (chưa từng sync) → luôn full sync, bất kể header content.
    // Tránh case 30 single-table syncs khi ensureSyncedIfEmpty fail/timeout.
    if (localGlobalVersion === 0) {
      await this.runFullSync(headers);
      return;
    }

    // Đã sync trước đó + BE chưa có per-table versions → no-op.
    if (Object.keys(serverVersions).length === 0) return;

    await this.ensureTableVersionsLoaded();
    const local = this.tableVersions ?? {};

    const mismatched: string[] = [];
    for (const [table, sv] of Object.entries(serverVersions)) {
      if (local[table] !== sv) mismatched.push(table);
    }
    if (mismatched.length === 0) return;

    // Sentinel "__all__" mismatch → handler legacy bump → full sync (FE không biết
    // table nào đổi). runFullSync sẽ tự persist versions từ response header.
    if (mismatched.includes('__all__')) {
      await this.runFullSync(headers);
      return;
    }

    // Per-table sync — chỉ table mismatch
    await Promise.all(mismatched.map((t) => this.syncTableOnce(t)));

    // Re-read this.tableVersions sau awaits để tránh race với handler concurrent khác.
    // Merge serverVersions vào latest in-memory state, không vào snapshot ban đầu.
    const latest = this.tableVersions ?? {};
    const updated = { ...latest, ...serverVersions };
    this.tableVersions = updated;
    await this.indexedDB.setTableVersions(updated).catch(() => {});

    // Tenant-defining tables (business_profile, branches) thay đổi → reload page để
    // toàn bộ UI (switch-branch, sidebar, scope filter, permission) refresh sạch.
    // Chỉ fire qua per-table path → KHÔNG loop khi boot (boot dùng runFullSync).
    const reloadTable = mismatched.find((t) => t in this.RELOAD_ON_CHANGE_TABLES);
    if (reloadTable) {
      // Form có thể đã set message cụ thể (create/update) — không overwrite.
      if (!sessionStorage.getItem('post_reload_toast')) {
        sessionStorage.setItem('post_reload_toast', this.RELOAD_ON_CHANGE_TABLES[reloadTable]);
      }
      // Defer reload 1 tick để response interceptor kịp deliver event cho subscriber
      // (form save callback có cơ hội close dialog trước reload).
      setTimeout(() => location.reload(), 0);
    }
  }

  /** Full sync với lock cross-tab + dedup in-tab */
  private async runFullSync(headers?: any): Promise<void> {
    if (this.syncPromise) return this.syncPromise;

    if (!this.acquireFullSyncLock()) {
      // Tab khác đang sync. Refresh in-memory cache từ IndexedDB để khi tab kia
      // hoàn thành (sẽ ghi vào IndexedDB shared), response tiếp theo của tab này
      // sẽ thấy versions match → không re-trigger sync nữa.
      const latest = await this.indexedDB.getTableVersions().catch(() => null);
      if (latest) this.tableVersions = latest;
      return;
    }

    this.syncPromise = this.performSync(headers ?? new Headers());
    try {
      await this.syncPromise;
    } finally {
      this.syncPromise = null;
      this.releaseFullSyncLock();
    }
  }

  private async performSync(headers: any): Promise<void> {
    try {
      const httpRes = await firstValueFrom(
        this.http.get<any>(`${environment.apiUrl}/crm/indexeddb/sync`, {
          context: new HttpContext().set(DEDUPE_DISABLE, true),
          observe: 'response',
        }),
      );

      await this.registry.syncAllTables(httpRes.body);
      await this.indexedDB.setLocalVersion(Math.floor(Date.now() / 1000));

      // Persist per-table versions từ response để future responses không re-trigger sync
      const tablesHeader = httpRes.headers.get('X-Dexie-Versions');
      if (tablesHeader) {
        const parsed = this.parseTableVersionsHeader(tablesHeader);
        this.tableVersions = parsed;
        await this.indexedDB.setTableVersions(parsed).catch(() => {});
      }
    } catch {
      // Không throw error để không block các request khác
    }
  }
}
