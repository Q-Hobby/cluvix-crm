import {Service} from "@angular/core";
import {BaseApiService} from "@/share/services/base-api.service";
import {Observable} from "rxjs";

@Service()
export class ReportCrmService extends BaseApiService<any> {
  protected override apiModule = 'crm';
  protected override endpoint = 'report';


  effectiveAdvise(params: any = {}) {
    return this.apiService.get(`${this.path}/effective-advise`, params);
  }

  telesalesOnline(params: any = {}) {
    return this.apiService.get(`${this.path}/telesales-online`, params);
  }

  revenue(params: any = {}) {
    return this.apiService.get(`${this.path}/revenue`, params);
  }

  /**
   * API trả về report theo nhóm khách hàng
   * @param params
   */
  getCategoryStatistics(params: any = {}) {
    return this.apiService.get(`${this.path}/category-statistics`, params);
  }

  /**
   * API trả về report theo nhóm dịch vụ quan tâm
   * @param params
   */
  getInterestedStatistics(params: any): Observable<any> {
    return this.apiService.get(`${this.path}/interested-statistics`, params);
  }

  /**
   * API trả về report theo quốc gia
   * @param params
   */
  getCountryStatistics(params: any = {}) {
    return this.apiService.get(`${this.path}/country-statistics`, params);
  }

  /**
   * API trả về report theo tỉnh / thành phố
   * @param params
   */
  getProvinceStatistics(params: any = {}) {
    return this.apiService.get(`${this.path}/province-statistics`, params);
  }

  getStatisticsAppointment(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/appointment-statistics`, params);
  }

  /**
   * API trả về report theo chi nhánh
   * @param params
   */
  getBranchStatistics(params: any = {}) {
    return this.apiService.get(`${this.path}/branch-statistics`, params);
  }

  /**
   * API trả về report theo nguồn
   * @param params
   */
  getSourceStatistics(params: any): Observable<any> {
    return this.apiService.get(`${this.path}/source-statistics`, params);
  }

  /**
   * API trả về report theo nhóm nguồn
   * @param params
   */
  getGroupSourceStatistics(params: any): Observable<any> {
    return this.apiService.get(`${this.path}/group-source-statistics`, params);
  }

  getServiceTypeStatistics(params: any): Observable<any> {
    return this.apiService.get(`${this.path}/service-type-statistics`, params);
  }

  getBranchAppointment(params: any = {}) {
    return this.apiService.get(`${this.path}/branch-appointment`, params);
  }
}
