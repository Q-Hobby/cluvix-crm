import { inject, Service} from '@angular/core';
import { ControlFieldInTableCacheService } from './control-field-in-table-cache.service';

@Service()
export class ControlFieldInTableHelperService {
  private cache = inject(ControlFieldInTableCacheService);

  async ensureControlFieldInTableLoaded(): Promise<void> {
    await this.cache.ensureControlFieldInTableLoaded();
  }

  getSmsTypeName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
