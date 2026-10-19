import { Service} from '@angular/core';
import { authUser } from '@/core/services/auth/auth.state';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface UserFilterOptions extends BaseCacheFilterOptions {
  department?: number;
  departments?: number[];
  allowedStatuses?: number[];
  branchId?: number;
  autoFallback?: boolean;
  includeUserId?: number;
  includeUserIds?: number[];
}

@Service()
export class UserCacheService extends BaseCacheService<UserProfileModel> {
  protected tableName = 'users';
  protected activeStatus = 10;
  protected allowedStatuses = [9, 10];
  private departmentIndex: Map<number, UserProfileModel[]> | null = null;

  private sortedCache: UserProfileModel[] | null = null;

  async loadUsers() {
    return this.loadItems();
  }

  getAll(): UserProfileModel[] {
    if (!this.sortedCache) {
      this.sortedCache = this.getAllItems()
        .filter((user) => this.allowedStatuses.includes(user.status!))
        .sort((a, b) => (a.fullname ?? '').localeCompare(b.fullname ?? ''));
    }
    return this.sortedCache;
  }

  getAllUsersUnfiltered(): UserProfileModel[] {
    return this.getAllItems();
  }

  filterUsers(options: UserFilterOptions): UserProfileModel[] {
    // Map to base interface
    const baseOptions = {
      ...options,
      includeItemId: options.includeUserId,
      includeItemIds: options.includeUserIds,
    };

    return this.filterItems(baseOptions, (users, opts) => {
      // Override status filtering với custom allowedStatuses
      const customOptions = opts as UserFilterOptions;
      const allowedStatuses = customOptions.allowedStatuses || this.allowedStatuses;

      if (customOptions.activeOnly) {
        users = users.filter((user) => user.status === this.activeStatus);
      } else {
        users = users.filter((user) => allowedStatuses.includes(user.status!));
      }

      return this.customUserFilter(users, customOptions);
    });
  }

  protected override onItemsUpdated(users: UserProfileModel[]): void {
    this.sortedCache = null;
    this.departmentIndex = null;
    this.buildDepartmentIndex(users);
  }

  protected override onCacheCleared(): void {
    this.sortedCache = null;
    this.departmentIndex = null;
  }

  protected override generateCacheKey(options: UserFilterOptions): string {
    return `${options.department || ''}_${(options.departments || []).join(',')}_${options.activeOnly}_${(options.allowedStatuses || []).join(',')}_${options.includeUserId || ''}_${(options.includeUserIds || []).join(',')}_${options.branchId || ''}`;
  }

  /** Build lại index department → users vô điều kiện — caller tự quyết khi nào cần (check null). */
  private buildDepartmentIndex(users: UserProfileModel[]) {
    this.departmentIndex = new Map();
    users.forEach((user) => {
      (user.departmentList || []).forEach((deptId) => {
        if (!this.departmentIndex!.has(deptId)) {
          this.departmentIndex!.set(deptId, []);
        }
        this.departmentIndex!.get(deptId)!.push(user);
      });
    });
  }

  private customUserFilter(
    users: UserProfileModel[],
    options: UserFilterOptions,
  ): UserProfileModel[] {
    // users đã được filter theo status ở trên
    // Auto fallback logic
    const { finalBranchId, finalBranchIds } = this.getAutoFallbackIds(options);

    // Phòng thủ: departmentIndex có thể bị onCacheCleared() xoá trong khi cachedItems còn ấm
    // (race loadItems vs getAllItems) → rebuild lazily thay vì lọc ra rỗng sai.
    if ((options.department || options.departments?.length) && !this.departmentIndex) {
      this.buildDepartmentIndex(this.getAllItems());
    }

    // Filter by department sử dụng index
    if (options.departments?.length) {
      const departmentUsers = new Set<UserProfileModel>();
      options.departments.forEach((deptId) => {
        const deptUsers = this.departmentIndex?.get(deptId) || [];
        deptUsers.forEach((user) => departmentUsers.add(user));
      });
      users = users.filter((user) => departmentUsers.has(user));
    } else if (options.department) {
      const deptUsers = this.departmentIndex?.get(options.department) || [];
      const deptUserSet = new Set(deptUsers);
      users = users.filter((user) => deptUserSet.has(user));
    }

    // Filter by branch - handle both single and multiple branches
    // Guard against stale IDB data where branchId may be a non-array (e.g. JSON null string)
    if (finalBranchId !== undefined && finalBranchId !== null) {
      users = users.filter((user) =>
        Array.isArray(user.branchId) && user.branchId.some((id) => +id === finalBranchId),
      );
    } else if (finalBranchIds?.length) {
      users = users.filter((user) =>
        Array.isArray(user.branchId) && user.branchId.some((id) => finalBranchIds!.includes(+id)),
      );
    }

    return users;
  }

  private getAutoFallbackIds(options: UserFilterOptions) {
    let finalBranchId = options.branchId;
    let finalBranchIds: number[] | undefined;

    if (options.autoFallback !== false) {
      const currentUser = authUser();

      // Nếu có branchId được truyền vào, sử dụng nó
      if (finalBranchId !== undefined && finalBranchId !== null) {
        // Sử dụng branchId được truyền vào
      } else if (currentUser) {
        // Auto fallback cho branchId khi không có giá trị được truyền vào
        const activeBranchId = currentUser.userProfile?.branchActive_id;
        const allowedBranchIds = currentUser.branchId || [];

        if (activeBranchId) {
          finalBranchId = activeBranchId;
        } else if (allowedBranchIds.length > 0) {
          finalBranchIds = allowedBranchIds;
        }
      }
    }

    // Ép id về number: branch_id/company_id từ API có thể là string, trong khi
    // user.branchId là number[] → so sánh strict +id === finalBranchId("5") luôn
    // false làm dropdown rỗng khi mở form edit. Giữ nguyên null/undefined để các
    // check `!= null` ở customUserFilter vẫn rơi vào nhánh fallback đúng.
    return {
      finalBranchId: this.toNum(finalBranchId),
      finalBranchIds: finalBranchIds?.map((id) => +id),
    };
  }

  private toNum(v: number | null | undefined): number | null | undefined {
    return v == null ? v : +v;
  }
}
