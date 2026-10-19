import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';

@Service()
export class AppointmentIsAcceptedCacheService extends BaseCacheService<AppointmentIsAcceptedModel> {
  protected tableName = 'appointmentIsAccepteds';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureAppointmentIsAcceptedsLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): AppointmentIsAcceptedModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
