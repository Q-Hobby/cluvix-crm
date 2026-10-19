import {ChangeDetectionStrategy, Component, inject, OnInit, ViewChild, signal} from '@angular/core';
import {Table} from 'primeng/table';
import {Observable, map} from 'rxjs';

import {BaseListV2} from '@/share/base/base-list-v2';
import {FilterFieldDef} from '@/share/base/list-filter.model';
import {ListApiResponse} from '@/share/models/list-api-response';
import {ListShareImports} from '@/share/import/list-share-imports';
import {ReportCrmService} from '@/core/services/report/report-crm.service';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-branch-appointment',
  templateUrl: './branch-appointment.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class BranchAppointment extends BaseListV2<any> implements OnInit {
  @ViewChild('dt') dt!: Table;
  modelService = inject(ReportCrmService);
  protected override URL_STATE_PREFIX = 'ba';

  totals = signal<any>({
    totalAppointment: 0,
    arrived: 0,
    cancelled: 0,
    agreed: 0,
    arrivedRate: 0,
    agreedPerArrived: 0,
    agreedRate: 0,
  });

  constructor() {
    super();
  }

  ngOnInit() {
    this.initListV2();
  }

  /**
   * Khai báo filter — `_created_at` có gạch dưới đầu → UtilsService.buildFilter tự xuất dạng
   * `_created_at[gte]`/`_created_at[lte]` (KHÔNG bọc `filter[]`), khớp đúng
   * `c.Query("_created_at[gte]")` BE đang đọc raw — giữ nguyên tên control, không đổi.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      {
        name: '_created_at',
        kind: 'dateRange',
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayStart()],
      },
      {name: 'appointment_type_id', kind: 'in'},
    ];
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.getBranchAppointment({params: this.searchParams()}).pipe(
      map((res: any) => {
        const items = (res.items ?? []).map((item: any) => ({
          branchId: item.branchId,
          branchName: item.branchName,
          totalAppointment: +item.totalAppointment || 0,
          arrived: +item.arrived || 0,
          cancelled: +item.cancelled || 0,
          agreed: +item.agreed || 0,
          arrivedRate: +item.totalAppointment ? (+item.arrived * 100) / +item.totalAppointment : 0,
          agreedPerArrived: +item.arrived ? (+item.agreed * 100) / +item.arrived : 0,
          agreedRate: +item.totalAppointment ? (+item.agreed * 100) / +item.totalAppointment : 0,
        }));

        const t = res.totals ?? {};
        const tTotal = +t.totalAppointment || 0;
        const tArrived = +t.arrived || 0;
        this.totals.set({
          totalAppointment: tTotal,
          arrived: tArrived,
          cancelled: +t.cancelled || 0,
          agreed: +t.agreed || 0,
          arrivedRate: tTotal ? (tArrived * 100) / tTotal : 0,
          agreedPerArrived: tArrived ? ((+t.agreed || 0) * 100) / tArrived : 0,
          agreedRate: tTotal ? ((+t.agreed || 0) * 100) / tTotal : 0,
        });

        return {
          ...res,
          items,
          _meta: res._meta ?? {totalCount: items.length, pageCount: 1, currentPage: 1, perPage: items.length},
        } as ListApiResponse<any>;
      }),
    );
  }

  onQuickSelect({start, end}: {start: Date; end: Date}) {
    this.rfFilterModel.patchValue({_created_at: [start, end]});
  }

  clearStartTime() {
    this.setFilterValue('_created_at', null);
  }
}
