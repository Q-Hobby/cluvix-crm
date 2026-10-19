import { Service} from '@angular/core';
import { AppointmentNotBookedReasonsModel } from '@/core/models/config/appointment/appointment-not-booked-reasons.model';
import { BaseCacheService } from '@/indexeddb/config/components/base';

@Service()
export class AppointmentNotBookedReasonsCacheService extends BaseCacheService<AppointmentNotBookedReasonsModel> {
  protected tableName = 'appointmentNotBookedReasons';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureReasonsLoaded(forceReload = false): Promise<void> {
    try {
      return await this.loadItems(forceReload);
    } catch (error) {
      console.warn(
        'Table appointmentNotBookedReasons not found, will be created on next DB upgrade',
      );
      // Return empty để component không crash
      return Promise.resolve();
    }
  }

  getAll(): AppointmentNotBookedReasonsModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.SortOrder || 0) - (b.SortOrder || 0));
  }

  getParentReasons(): AppointmentNotBookedReasonsModel[] {
    return this.getAllItems()
      .filter(
        (item) =>
          item.status === 1 &&
          (item.parent_id === 0 || item.parent_id === null || item.parent_id === undefined),
      )
      .sort((a, b) => (a.SortOrder || 0) - (b.SortOrder || 0));
  }

  getChildReasons(parentId: number): AppointmentNotBookedReasonsModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1 && item.parent_id === parentId && parentId !== 0)
      .sort((a, b) => (a.SortOrder || 0) - (b.SortOrder || 0));
  }

  getCascadeOptions(): any[] {
    try {
      const parents = this.getParentReasons();

      const result = parents
        .map((parent) => {
          const children = this.getChildReasons(parent.id!);

          return {
            name: parent.name,
            id: parent.id,
            children: children.map((child) => ({
              name: child.name,
              id: child.id,
            })),
          };
        })
        .filter((item) => item.children && item.children.length > 0);
      return result;
    } catch (error) {
      console.warn('Error getting cascade options:', error);
      return [];
    }
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
