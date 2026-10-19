import { Service } from '@angular/core';

import { hasPermission } from '@/core/services/auth/auth.selectors';

/**
 * Thay thế NgxPermissionsService (đã bỏ dùng) — giữ nguyên API `getPermission(name)`
 * để không phải sửa hàng trăm call site `this.permissionsService.getPermission(...)`.
 * Nguồn thật là signal `userPermissions` (xem auth.selectors.ts / auth.state.ts).
 */
@Service()
export class PermissionsService {
  getPermission(name: string): boolean {
    return hasPermission(name);
  }
}
