import { computed, Service} from '@angular/core';
import { branchActiveId } from '@/core/services/auth';

type TenantHook = () => Promise<void> | void;

@Service()
export class TenantService {
  readonly currentBranchId = computed(() => branchActiveId());

  private readonly hooks: TenantHook[] = [];

  register(hook: TenantHook): void {
    this.hooks.push(hook);
  }

  async dispatchTenantChange(): Promise<void> {
    for (const hook of this.hooks) {
      await hook();
    }
  }
}
