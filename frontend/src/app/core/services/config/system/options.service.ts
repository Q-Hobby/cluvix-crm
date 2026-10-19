import { Service} from '@angular/core';
import { HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { UserOptionModel } from '@/core/models/config/system/user-option.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class OptionsService extends BaseApiService<UserOptionModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'options';

  // ids: optional filter — không truyền vẫn trả toàn bộ (option-crm.ts cần full list, match theo
  // cả `name` lẫn `id`, không thể filter trước theo id cố định).
  getOptions(ids?: number[]) {
    const options = ids?.length ? { params: new HttpParams().set('ids', ids.join(',')) } : {};
    return this.apiService.get(`${this.path}/get-options`, options);
  }

  optionUpdate(id: number, options: any = {}): Observable<UserOptionModel> {
    return this.apiService.put<any>(`${this.path}/update-option/${id}`, options).pipe(map((res) => res?.data ?? res));
  }
}
