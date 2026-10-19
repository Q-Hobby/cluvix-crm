import { inject, Service} from '@angular/core';
import { UserCacheService } from './user-cache.service';

@Service()
export class UserHelperService {
  private userCache = inject(UserCacheService);

  /**
   * Tên HIỂN THỊ NỘI BỘ — ưu tiên BIỆT DANH (`nickname`), fallback `fullname`.
   * Dùng cho teamchat, log, panel nội bộ… KHÔNG dùng cho giấy tờ pháp lý/chữ ký
   * (xem `getFullName`).
   */
  getLabel(id: number | null | undefined): string {
    if (!id) return '';
    const user = this.userCache.getById(id);
    return user?.nickname || user?.fullname || '';
  }

  /**
   * Tên ĐẦY ĐỦ theo hồ sơ nhân sự (`fullname`), fallback `nickname` chỉ khi hồ sơ
   * chưa có họ tên.
   *
   * ⚠ Dùng cho GIẤY TỜ PHÁP LÝ / Ô CHỮ KÝ (phiếu Bộ Y tế ghi "(Ký, ghi rõ họ tên)")
   * — khác `getLabel()` vốn ưu tiên biệt danh cho hiển thị nội bộ. Các phiếu snapshot
   * tên người ký vào JSON là BẤT BIẾN: ghi nhầm biệt danh ("Hà NV" thay cho
   * "Nguyễn Thị Hà") sẽ sai vĩnh viễn trên bản in.
   */
  getFullName(id: number | null | undefined): string {
    if (!id) return '';
    const user = this.userCache.getById(id);
    return user?.fullname || user?.nickname || '';
  }

  getUserInfo(id: number | null | undefined): {
    fullname: string;
    nickname: string;
    avatar: string;
    status: number;
    branchId: number[];
    department: number[];
  } | null {
    if (!id) return null;
    const user = this.userCache.getById(id);
    if (!user) return null;

    return {
      fullname: user.nickname || user.fullname || '',
      nickname: user.nickname ?? '',
      avatar: user.avatar ?? '',
      status: user.status ?? 0,
      branchId: user.branchId ?? [],
      department: user.departmentList ?? [],
    };
  }

  mapUserNames(ids: number[]): string[] {
    if (!ids || ids.length === 0) return [];
    return ids.map((id) => this.userCache.getById(id)?.fullname || '').filter((name) => name);
  }

  async ensureUsersLoaded() {
    await this.userCache.loadUsers();
  }

  private toTitleCase(str: string): string {
    if (!str) return '';
    return str
      .toLowerCase()
      .split(' ')
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  }
}
