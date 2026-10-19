import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class LocationCountryService extends BaseApiService<LocationCountryModel> {
  protected override apiModule = 'config';
  protected override endpoint: string = 'location-country';

  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    // Signal Forms: mọi field text `pInputText` seed '' (gotcha #9), KHÔNG `null`.
    return {
      CountryCode: '',
      CommonName: '',
      FormalName: '',
      Capital: '',
      TelephoneCode: '',
      CurrencyCode: '',
      CurrencyName: '',
      status: 1,
    };
  }

  loadCountryByKeyword($keyWord: any): Observable<any> {
    return this.apiService.get(`${this.path}/load-country-by-keyword?keyword=${$keyWord}`);
  }

  loadAllForDropdown(): Observable<any> {
    return this.apiService.get(`${this.path}/load-all-for-dropdown`);
  }
}
