import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class DashboardService extends BaseApiService<any> {
  protected override endpoint = 'report';

  /**
   * API báo cáo lịch hẹn tháng này so với tháng trước
   */
  compareappointment(): Observable<any> {
    return this.apiService.get(`${this.path}/compare-appointment`);
  }

}
