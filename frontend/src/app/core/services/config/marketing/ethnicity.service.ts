import { Service} from '@angular/core';
import { BaseApiService } from '@/share/services/base-api.service';
import { EthnicityModel } from '@/core/models/config/marketing/ethnicity.model';

@Service()
export class EthnicityService extends BaseApiService<EthnicityModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'ethnicity';
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    return {
      name: '',
      codeEthnicity: '',
      status: 1,
    };
  }
}
