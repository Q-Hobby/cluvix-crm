import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { NoteTemplateForm } from '@/modules/config/config-customer/note-template/form/note-template-form';
import { NoteTemplateModel } from '@/core/models/config/marketing/note-template.model';
import { NoteTemplateService } from '@/core/services/config/marketing/note-template.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-note-template-list',
  templateUrl: './note-template-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class NoteTemplateList extends BaseListV2<NoteTemplateModel> implements OnInit {
  @ViewChild('dt') dt!: Table<NoteTemplateModel>;
  protected override URL_STATE_PREFIX = 'nt';
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Tên mẫu', labelKey: 'config.note_template.name', value: 'name' },
    { label: 'Nội dung', labelKey: 'config.note_template.content', value: 'content' },
    { label: 'Loại mẫu template', labelKey: 'common.label.template_type', value: 'type' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'content', 'name', 'type', 'status', 'SortOrder']);

  /** START CRUD */
  override formComponent = NoteTemplateForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  modelService = inject(NoteTemplateService);

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
   * Khai báo filter — list này chỉ có ô tìm kiếm theo từ khoá.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }
}
