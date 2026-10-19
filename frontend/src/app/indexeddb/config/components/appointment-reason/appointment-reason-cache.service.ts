import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { AppointmentReasonModel } from '@/core/models/config/appointment/appointment-reason.model';

@Service()
export class AppointmentReasonCacheService extends BaseCacheService<AppointmentReasonModel> {
  protected tableName = 'appointmentReasons';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureAppointmentReasonLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): AppointmentReasonModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
