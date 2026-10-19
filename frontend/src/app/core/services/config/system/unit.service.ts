import { inject, Service} from '@angular/core';

import { UnitModel } from '@/core/models/config/system/unit.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { UNIT_CATEGORY_MAP } from '@/define';
import { Observable } from 'rxjs';
import { catchApiError } from '@/share/helpers/catch-api-error';

@Service()
export class UnitService extends BaseApiService<UnitModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'unit';
  public readonly TOOTH_ID: number = 1;
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    return {
      name: '',
      description: '',
      status: 1,
      categoryId: [] as number[],
    };
  }

  override importData(params: any) {
    const formData: FormData = new FormData();
    formData.append('file', params, params.name);
    return this.apiService
      .post(`${this.path}/import-from-excel`, formData)
      .pipe(catchApiError());
  }

  exportUnit(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}?_export=1`, params);
  }

  addLabel(item: UnitModel) {
    const categoryIds = Array.isArray(item.categoryId) ? item.categoryId : [];
    return {
      ...item,
      categoryInfo: categoryIds.map((id) => UNIT_CATEGORY_MAP[id]).filter((c) => c),
    };
  }

  async addLabels(items: UnitModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    // Không còn cache nào phụ thuộc company cần ensure trước khi addLabel.
  }
}
