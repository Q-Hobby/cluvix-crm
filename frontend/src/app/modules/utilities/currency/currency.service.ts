import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class CurrencyService extends BaseApiService<any> {
  public override readonly title: string = 'Chuyển đổi';
  protected override endpoint = 'exchange-rate';

  convertCurrency($params: any): Observable<any> {
    return this.apiService.get(
      `${this.path}/currency?from=${$params['from']}&num=${$params['num']}&to=${$params['to']}`,
    );
  }
}
