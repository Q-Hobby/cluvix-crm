import { inject, Service} from '@angular/core';

import { ExportHistoryModel } from '@/core/models/history/export-history.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { UserHelperService } from '@/indexeddb/config/components/user';

@Service()
export class ExportHistoryService extends BaseApiService<ExportHistoryModel> {
  protected override apiModule = 'history';
  protected override endpoint = 'history-export';
  protected override unixTimesTampFields = ['createdAt'];
  private userHelper: UserHelperService = inject(UserHelperService);

  addLabel(item: ExportHistoryModel) {
    const itemWithDates = this.addFormattedTimestampFields(item, ['createdAt']);
    return {
      ...itemWithDates,
      createdByFormat: this.userHelper.getUserInfo(item.createdBy),
    };
  }

  async addLabels(items: ExportHistoryModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([this.userHelper.ensureUsersLoaded()]);
  }
}
