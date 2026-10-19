import { Service} from '@angular/core';

import { SourceModel } from '@/core/models/config/marketing/source.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class SourceService extends BaseApiService<SourceModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'source';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    return {
      name: '',
      status: 1,
    };
  }

  addLabel(item: SourceModel) {
    return { ...item };
  }

  async addLabels(items: SourceModel[]) {
    if (!Array.isArray(items)) return [];
    return items.map((item) => this.addLabel(item));
  }
}
