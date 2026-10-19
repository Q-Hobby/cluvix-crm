import { inject, Service} from '@angular/core';
import { AppointmentNotBookedReasonsCacheService } from './appointment-not-booked-reasons-cache.service';

@Service()
export class AppointmentNotBookedReasonsHelperService {
  private cache = inject(AppointmentNotBookedReasonsCacheService);

  async ensureReasonsLoaded(): Promise<void> {
    await this.cache.ensureReasonsLoaded();
  }

  getReasonName(id: number | null | undefined): string {
    if (!id) return '—';
    const item = this.cache.getById(id);
    return item?.name || '—';
  }
}
