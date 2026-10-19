import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { branchListId } from '@/core/services/auth/auth.selectors';
import { BranchModel } from '@/core/models/config/system/branch.model';

export interface BranchFilterOptions extends BaseCacheFilterOptions {
  includeBranchId?: number;
  includeBranchIds?: number[];
}

@Service()
export class BranchCacheService extends BaseCacheService<BranchModel> {
  protected tableName = 'branches';
  protected activeStatus = 1; // Giả sử branch không có status field, dùng default
  protected allowedStatuses = [1];

  async loadBranches() {
    return this.loadItems();
  }

  getAllBranchesUnfiltered(): BranchModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  filterBranches(options: BranchFilterOptions): BranchModel[] {
    const baseOptions = {
      ...options,
      includeItemId: options.includeBranchId,
      includeItemIds: options.includeBranchIds,
    };

    return this.filterItems(baseOptions, (branches, opts) =>
      this.customBranchFilter(branches, opts as BranchFilterOptions),
    );
  }

  filterBranchesByUser(options: BranchFilterOptions = {}): BranchModel[] {
    const userBranchIds = branchListId();
    const allBranches = this.filterBranches(options);
    return allBranches.filter((branch) => userBranchIds.includes(branch.id!));
  }

  protected override generateCacheKey(options: BranchFilterOptions): string {
    const userBranchIds = branchListId().join(',');
    return `${options.activeOnly}_${options.includeBranchId || ''}_${(options.includeBranchIds || []).join(',')}_${userBranchIds}`;
  }

  private customBranchFilter(branches: BranchModel[], options: BranchFilterOptions): BranchModel[] {
    // Filter by active status
    if (options.activeOnly) {
      branches = branches.filter((branch) => branch.status === this.activeStatus);
    }

    return branches;
  }
}
