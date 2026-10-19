import { inject, Service} from '@angular/core';
import { AppointmentTypeCacheService } from './appointment-type-cache.service';

@Service()
export class AppointmentTypeHelperService {
  private cache = inject(AppointmentTypeCacheService);

  async ensureAppointmentTypesLoaded(): Promise<void> {
    await this.cache.ensureAppointmentTypesLoaded();
  }

  getAppointmentTypeName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
