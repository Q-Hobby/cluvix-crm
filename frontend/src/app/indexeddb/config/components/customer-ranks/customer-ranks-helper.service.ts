import { inject, Service} from '@angular/core';
import { CustomerRanksCacheService } from '@/indexeddb/config/components/customer-ranks/customer-ranks-cache.service';

@Service()
export class CustomerRanksHelperService {
  private cache = inject(CustomerRanksCacheService);

  async ensureCustomerRanksLoaded(): Promise<void> {
    await this.cache.ensureCustomerRanksLoaded();
  }

  getCustomerRankInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
      name: item.name,
    };
  }

  getRankById(id: number) {
    return this.cache.getById(id);
  }
}
