import {inject, Service} from '@angular/core';
import {Observable, map, tap} from 'rxjs';

import {STORAGE_KEY_SELECT} from '@/define';
import {PermissionItem} from '@/share/interface/configData';
import {StorageService} from '@/share/services/storage.service';
import {BaseApiService} from '@/share/services/base-api.service';
import {userPermissions} from './auth.state';
import {hasPermission as checkPermission} from './auth.selectors';

@Service()
export class RbacService extends BaseApiService<any> {
  protected override apiModule = 'auth';
  protected override endpoint = 'rbac';
  private storageService = inject(StorageService);

  /**
   * API trả về danh sách permission theo role
   * @param roleName
   */
  getListPermissionByRole(roleName: string): Observable<string[]> {
    return this.apiService.get(`${this.path}/list-permission?roleName=${roleName}`);
  }

  /**
   * Lấy ra dropdown nhóm vai trò của user login, không dùng được indexeddb vì mỗi user có danh sách khác nhau
   */
  getChildRoleForDropdown() {
    return this.apiService.get(`${this.path}/child-role-for-dropdown`);
  }

  setPermissions(permissions: PermissionItem[]): void {
    const arr = this.toStringArray(permissions);
    userPermissions.set(new Set(arr));
    this.storageService.saveStorage(STORAGE_KEY_SELECT.ConfigPermission, permissions);
  }

  /** Load permissions từ string[] (IndexedDB path) — không ghi localStorage. */
  setPermissionsFromStrings(arr: string[]): void {
    userPermissions.set(new Set(arr));
  }

  getPermissions(): PermissionItem[] {
    return (this.storageService.loadStorage(STORAGE_KEY_SELECT.ConfigPermission) as PermissionItem[]) || [];
  }

  getPermissionStrings(): string[] {
    return this.toStringArray(this.getPermissions());
  }

  restorePermissions(): void {
    const items = this.getPermissions();
    if (items.length) {
      this.setPermissions(items);
    }
  }

  clearPermissions(): void {
    userPermissions.set(new Set());
    this.storageService.removeStorage(STORAGE_KEY_SELECT.ConfigPermission);
  }

  hasPermission(permissionName: string): boolean {
    return checkPermission(permissionName);
  }

  /**
   * Lấy danh sách permissions từ API
   */
  fetchPermissions(): Observable<any> {
    return this.apiService.get(`${this.path}/permissions`);
  }

  /**
   * Lấy danh sách roles từ API
   */
  fetchRoles(): Observable<any> {
    return this.apiService.get(`${this.path}/roles`);
  }

  // --- Role management (từ RoleService) ---

  createRole(createParams: any) {
    return this.apiService.post(`${this.path}/role/create-name`, createParams);
  }

  deleteRole(name: string): Observable<any> {
    return this.apiService.delete(`${this.path}/role/delete-name?name=${name}`);
  }

  updateRole(name: string, updateParams: any): Observable<any> {
    return this.apiService.put(`${this.path}/role/update-name?name=${name}`, updateParams);
  }

  // Chỉ dùng bởi `RoleForm` (form duy nhất dùng RbacService làm `modelService`) — seed đủ key cho
  // Signal Forms (gotcha #1 angular22.md, field text bind `pInputText` phải seed `''`, KHÔNG `null`/
  // `undefined`). KHÔNG ảnh hưởng consumer khác của RbacService (default `getInitialObject()` của
  // `BaseApiService` chỉ trả `{}`, không có nơi nào khác gọi trực tiếp method này).
  override getInitialObject(): { name: string; description: string } {
    return { name: '', description: '' };
  }

  getRole(name: string): Observable<any> {
    return this.apiService.get(`${this.path}/role/get-name?name=${name}`).pipe(
      map((res: any) => res?.data ?? res),
    );
  }

  updatePermissionWithRole(roleName: string, listPer: any[]) {
    return this.apiService.post(
      `${this.path}/update-permission-with-role?roleName=${roleName}`,
      { listPer },
    );
  }

  /**
   * Sync permissions từ API và lưu local
   */
  syncPermissions(): Observable<PermissionItem[]> {
    return this.fetchPermissions().pipe(
      tap((permissions: PermissionItem[]) => this.setPermissions(permissions)),
    );
  }

  // --- Permission entity management ---

  createPermission(createParams: any) {
    return this.apiService.post(`${this.path}/permission/create-name`, createParams);
  }

  deletePermission(name: string): Observable<any> {
    return this.apiService.delete(`${this.path}/permission/delete-name?name=${name}`);
  }

  updatePermission(name: string, updateParams: any): Observable<any> {
    return this.apiService.put(`${this.path}/permission/update-name?name=${name}`, updateParams);
  }

  getPermissionByName(name: string): Observable<any> {
    return this.apiService.put(`${this.path}/permission/get-name?name=${name}`);
  }

  public toStringArray(permissions: PermissionItem[]): string[] {
    if (!Array.isArray(permissions)) return [];
    return permissions
      .map((p: any) => (typeof p === 'string' ? p : p?.name || p?.code || p?.key || String(p)))
      .filter(Boolean);
  }

}
