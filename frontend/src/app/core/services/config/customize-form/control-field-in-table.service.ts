import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { CustomizeFormModel } from '@/core/models/config/customize-form/customize-form.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class ControlFieldInTableService extends BaseApiService<CustomizeFormModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'control-field-in-table';
  readonly CUSTOMIZE_FORM_ID = 1;
  readonly CUSTOMIZE_FORM_LEAD_ID = 2;

  /**
   * Load field customize form
   */
  getFields(id: number) {
    return this.apiService.get(`${this.path}/${id}/get-field-customer`);
  }

  /**
   * Save field customize form
   * @param id
   * @param payload
   */
  saveFields(id: number, payload: any): Observable<any> {
    return this.apiService.post(`${this.path}/${id}/update-field-customer`, payload);
  }

  /**
   * Rebuild customize form
   * @param id
   */
  reBuild(id: number): Observable<any> {
    return this.apiService.post(`${this.path}/${id}/re-build`);
  }
}
