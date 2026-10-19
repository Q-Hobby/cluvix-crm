import { Service} from '@angular/core';
import { STORAGE_KEY_SELECT } from '@/define';

// Lang KHÔNG nằm trong danh sách này — đây là preference của thiết bị/trình duyệt,
// không phải dữ liệu auth theo tài khoản, nên phải sống sót qua logout (bug 2026-07-23:
// clear cùng lúc khiến chọn tiếng Anh ở màn login xong logout ra lại về tiếng Việt mặc định).
const LOGOUT_KEYS: string[] = [
  'access_token',
  'refresh_token',
  'token_expires_at',
  'refresh_token_expires_at',
  'refresh_lock',
  'auth_user_id',
  STORAGE_KEY_SELECT.AuthUser,
  STORAGE_KEY_SELECT.ConfigDataDropdown,
  STORAGE_KEY_SELECT.ConfigPermission,
  STORAGE_KEY_SELECT.ConfigMenuRecent,
  STORAGE_KEY_SELECT.KEY_ROWS_TABLE,
  STORAGE_KEY_SELECT.LayoutConfig,
  STORAGE_KEY_SELECT.Recent,
  STORAGE_KEY_SELECT.Favourite,
  STORAGE_KEY_SELECT.ScheduleViewMode,
  STORAGE_KEY_SELECT.CrmDashboardLayout,
];

@Service()
export class StorageService {
  constructor() {}

  /**
   * Lưu dữ liệu vào localStorage
   * @param key
   * @param value
   * @param raw
   */
  saveStorage<T = any>(key: string, value: T, raw: boolean = false): void {
    try {
      const str = raw ? (value as any).toString() : JSON.stringify(value);
      localStorage.setItem(key, str);
    } catch (e) {
      console.error('Error saving to storage', e);
    }
  }

  /**
   * Lấy dữ liệu từ localStorage
   * @param key
   * @param autoParse
   */
  loadStorage<T = any>(key: string, autoParse: boolean = true): T | string | null {
    try {
      const str = localStorage.getItem(key);
      if (!str || str === 'undefined') return null;

      if (autoParse) {
        return JSON.parse(str);
      }

      return str;
    } catch (e) {
      console.error('Error loading from storage', e);
      return null;
    }
  }

  /**
   * Xóa 1 key trong localStorage
   * @param key
   */
  removeStorage(key: string): void {
    try {
      localStorage.removeItem(key);
    } catch (e) {
      console.error('Error removing from storage', e);
    }
  }

  clearOnLogout(): void {
    LOGOUT_KEYS.forEach(k => localStorage.removeItem(k));
  }
}
