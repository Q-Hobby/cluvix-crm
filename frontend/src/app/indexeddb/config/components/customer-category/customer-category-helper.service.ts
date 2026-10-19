import { inject, Service} from '@angular/core';
import { CustomerCategoryCacheService } from './customer-category-cache.service';

@Service()
export class CustomerCategoryHelperService {
  private cache = inject(CustomerCategoryCacheService);

  async ensureCustomerCategoryLoaded(): Promise<void> {
    await this.cache.ensureCustomerCategorysLoaded();
  }

  getCustomerCategoryName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
