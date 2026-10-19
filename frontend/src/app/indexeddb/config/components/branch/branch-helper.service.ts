import { inject, Service} from '@angular/core';
import { BranchCacheService } from './branch-cache.service';

@Service()
export class BranchHelperService {
  private branchCache = inject(BranchCacheService);

  getBranchName(id: number | null | undefined): string {
    if (!id) return '';
    const branch = this.branchCache.getById(id);
    return branch?.name ?? '';
  }

  getBranchInfo(id: number | null | undefined): {
    name: string;
    code: string;
    hotline: string;
    email: string;
    address: string;
  } | null {
    if (!id) return null;

    const branch = this.branchCache.getById(id);
    if (!branch) return null;

    return {
      name: branch.name!,
      code: branch.code!,
      hotline: branch.hotline ?? '',
      email: branch.email ?? '',
      address: branch.address ?? '',
    };
  }

  // Map array ids thành names
  mapBranchNames(ids: number[]): string[] {
    if (!Array.isArray(ids)) return [];
    return ids
      .filter((id) => id != null)
      .map((id) => this.branchCache.getById(id)?.name || '')
      .filter((name) => name);
  }

  // Ensure branches are loaded
  async ensureBranchesLoaded() {
    await this.branchCache.loadBranches();
  }
}
