import {ChangeDetectionStrategy, Component, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { SettingService } from '@/core/services/config/system/setting.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { SettingModel } from '@/core/models/config/system/setting.model';
import { ListShareImports } from '@/share/import/list-share-imports';
import { PERMISSION_SELECT } from '@/define';
import { Observable, finalize } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-help-setting',
  templateUrl: './setting.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class Setting extends BaseListV2<SettingModel> implements OnInit {
  modelService = inject(SettingService);
  @ViewChild('dt') dt!: Table<SettingModel>;
  protected override URL_STATE_PREFIX = 'set';

  constructor() {
    super();
  }

  ngOnInit() {
    this.initListV2();
  }

  /**
   * List này KHÔNG có filter panel (không có `rfFilterModel`/ô lọc nào trong template) →
   * miễn trừ AC-2/3/7/8 của story (chỉ áp AC-1: chuyển sang BaseListV2 + 5 binding bắt buộc +
   * thanh progress). Trả mảng rỗng — base vẫn cần override abstract method này.
   */
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  /**
   * Giữ nguyên loadingService riêng (`loadingSetting`) của V1 — không phải phần base lo, nên
   * vẫn override `fetchList` để bọc thêm show/hide loading như cũ.
   */
  protected override fetchList(): Observable<ListApiResponse<SettingModel>> {
    this.loadingService.showLoading('loadingSetting');
    return this.modelService.list({ params: this.searchParams() }).pipe(
      finalize(() => {
        this.loadingService.hideLoading('loadingSetting');
      }),
    );
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }
}
