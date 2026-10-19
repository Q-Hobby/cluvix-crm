import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';

import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { SourceHelperService } from '@/indexeddb/config/components/source';

@Service()
export class GroupSourceService extends BaseApiService<GroupSourceModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'group-source';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];
  private sourceHelper = inject(SourceHelperService);

  override getInitialObject() {
    return {
      name: '',
      desc: '',
      status: 1,
      online: 0,
      sourceId: [],
    };
  }

  checkOnline(params: any): Observable<string> {
    return this.apiService.post(`${this.path}/check-online`, params, {});
  }

  addLabel(item: GroupSourceModel) {
    return {
      ...item,
      sourceInfo: this.sourceHelper.mapSourceInfos(item.sourceId || []),
    };
  }

  async addLabels(items: GroupSourceModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.sourceHelper.ensureSourcesLoaded(),
    ]);
  }
}
