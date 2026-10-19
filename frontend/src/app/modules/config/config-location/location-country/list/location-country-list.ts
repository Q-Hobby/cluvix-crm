import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { LocationCountryForm } from '@/modules/config/config-location/location-country/form/location-country-form';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { LocationCountryService } from '@/core/services/config/location/location-country.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT, STATUS_OPTION } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-location-country',
  templateUrl: './location-country-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class LocationCountryList
  extends BaseListV2<LocationCountryModel>
  implements OnInit
{
  @ViewChild('dt') dt!: Table<LocationCountryModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Quốc gia', labelKey: 'common.label.country', value: 'CommonName' },
    { label: 'Mã quốc gia', labelKey: 'config.location.location_country.code', value: 'CountryCode' },
    { label: 'Thủ đô', labelKey: 'config.location.location_country.capital', value: 'Capital' },
    { label: 'Mã điện thoại', labelKey: 'config.location.location_country.telephone_code', value: 'TelephoneCode' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
  ];
  override selectedColumns = signal([
    'id',
    'CommonName',
    'CountryCode',
    'Capital',
    'TelephoneCode',
    'SortOrder',
    'status',
  ]);
  /** START CRUD */
  override formComponent = LocationCountryForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  modelService = inject(LocationCountryService);
  protected readonly STATUS_OPTION = STATUS_OPTION;
  protected override URL_STATE_PREFIX = 'lc';

  constructor() {
    super();
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 2);
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  /**
   * Khai báo filter. `status` giữ nguyên `kind: 'in'` (multiselect) đúng hành vi V1
   * (`fb.control([])` + `dt.filter(value, 'status', 'in')`) — dù BE `ListLocationCountry`
   * chỉ đọc `c.Query("filter[status]")` dạng scalar bare-key, trong khi FE với `kind:'in'`
   * gửi key có index (`filter[status][0]=...`) nên control này đã CHẾT SẴN TỪ V1 (BE
   * không bao giờ nhận được giá trị). Không tự đổi hành vi — xem `deferred-work.md`.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      { name: 'status', kind: 'in' },
    ];
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }
}
