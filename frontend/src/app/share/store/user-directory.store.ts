// user-directory.store.ts
import { computed, Service, signal } from '@angular/core';

import { STORAGE_KEY_SELECT } from '@/define';

export type UserOption = {
  value: number; // id user
  label: string; // tên
  nickname?: string | null;
  avatar?: string | null;
  status?: number;
  branchId?: number[] | null;
  departmentList?: number[] | null;
};

type DropdownEntry = {
  field_name: string;
  options: UserOption[];
  version?: string;
};

@Service()
export class UserDirectoryStore {
  // nếu bạn có version cho dropdown → set vào đây
  readonly version = signal<string>('');
  // Chính là key bạn đang dùng trong AuthService
  private readonly LS_KEY = STORAGE_KEY_SELECT.ConfigDataDropdown;
  private readonly FIELD_NAME = 'user';
  private _options = signal<UserOption[]>([]);
  readonly options = this._options;
  private _map = computed(() => {
    const m = new Map<number, UserOption>();
    for (const o of this._options()) {
      // phòng khi value bị stringify
      const id = typeof o.value === 'string' ? Number(o.value) : o.value;
      if (Number.isFinite(id)) m.set(id, o);
    }
    return m;
  });

  constructor() {
    this.loadFromLocalStorage();
    // Đồng bộ cross-tab
    window.addEventListener('storage', (e) => {
      if (e.key === this.LS_KEY) this.loadFromLocalStorage();
    });
  }

  /** Gọi lại hàm này sau khi AuthService.setDataDropdown(...) */
  loadFromLocalStorage() {
    try {
      const raw = localStorage.getItem(this.LS_KEY);
      if (!raw) return;

      const parsed = JSON.parse(raw);
      // Trường hợp A: là MẢNG các dropdown
      if (Array.isArray(parsed)) {
        const entry = (parsed as DropdownEntry[]).find((d) => d?.field_name === this.FIELD_NAME);
        if (entry?.options?.length) {
          this._options.set(entry.options);
          this.version.set(entry.version ?? '');
          return;
        }
      }
      // Trường hợp B: là OBJECT { field_name, options }
      if (parsed?.field_name === this.FIELD_NAME && parsed?.options?.length) {
        this._options.set(parsed.options);
        this.version.set(parsed.version ?? '');
      }
    } catch {
      // ignore
    }
  }

  /** Lấy tên hiển thị: ưu tiên nickname, fallback label */
  name(id?: number | string | null, fallback = '—'): string {
    const nid = this.coerceId(id);
    if (nid === null) return fallback;
    const u = this._map().get(nid);
    return (u?.nickname ?? u?.label) || fallback;
  }

  avatar(id?: number | string | null): string | null {
    const nid = this.coerceId(id);
    if (nid === null) return null;
    return this._map().get(nid)?.avatar ?? null;
  }

  private coerceId(id?: number | string | null): number | null {
    if (id === null || id === undefined) return null;
    const n = typeof id === 'string' ? Number(id) : id;
    return Number.isFinite(n) ? n : null;
  }
}
