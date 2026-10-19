import { inject, Service} from '@angular/core';
import { CustomerRankServicesCacheService } from '@/indexeddb/config/components/customer-rank-services/customer-rank-services-cache.service';

@Service()
export class CustomerRankServicesHelperService {
  private cache = inject(CustomerRankServicesCacheService);

  async ensureCustomerRankServicesLoaded(): Promise<void> {
    await this.cache.ensureCustomerRankServicesLoaded();
  }

  getCustomerRankServicesInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
    };
  }

  getRankServiceByRankAndService(rankId: number, serviceId: number) {
    const items = this.cache.getAll();
    const matchingItems = items.filter((item) => {
      if (item.rank_id !== rankId || !item.service_ids) return false;

      try {
        const serviceIds =
          typeof item.service_ids === 'string' ? JSON.parse(item.service_ids) : item.service_ids;
        return Array.isArray(serviceIds) && serviceIds.includes(serviceId);
      } catch {
        return false;
      }
    });

    // Trả về record mới nhất (id lớn nhất)
    return matchingItems.length > 0
      ? matchingItems.reduce((latest, current) => ((current?.id ?? 0) > (latest?.id ?? 0) ? current : latest))
      : undefined;
  }

  getAll() {
    return this.cache.getAll();
  }
}
