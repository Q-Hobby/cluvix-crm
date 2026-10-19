import { inject, Service} from '@angular/core';
import { AppointmentReasonCacheService } from '@/indexeddb/config/components/appointment-reason/appointment-reason-cache.service';

@Service()
export class AppointmentReasonHelperService {
  private cache = inject(AppointmentReasonCacheService);

  async ensureAppointmentReasonLoaded(): Promise<void> {
    await this.cache.ensureAppointmentReasonLoaded();
  }

  getAppointmentReasonInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
      name: item.name,
    };
  }
}
