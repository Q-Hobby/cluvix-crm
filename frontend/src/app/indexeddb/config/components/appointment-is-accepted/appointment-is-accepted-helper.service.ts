import { inject, Service} from '@angular/core';
import { AppointmentIsAcceptedCacheService } from './appointment-is-accepted-cache.service';

@Service()
export class AppointmentIsAcceptedHelperService {
  private cache = inject(AppointmentIsAcceptedCacheService);

  async ensureAppointmentIsAcceptedsLoaded(): Promise<void> {
    await this.cache.ensureAppointmentIsAcceptedsLoaded();
  }

  getAppointmentIsAcceptedInfo(
    id: number | null | undefined,
  ): { name: string; severity: string } | null {
    if (!id) return null;
    const item = this.cache.getById(id);
    return item ? { name: item.name!, severity: item.severity! } : null;
  }
}
