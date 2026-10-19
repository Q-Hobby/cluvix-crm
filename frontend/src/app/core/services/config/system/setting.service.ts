import { Service} from '@angular/core';

import { SettingModel } from '@/core/models/config/system/setting.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class SettingService extends BaseApiService<SettingModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'setting';
}
