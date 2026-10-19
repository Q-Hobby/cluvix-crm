import { ChangeDetectionStrategy, Component, inject, OnInit, signal, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Table } from 'primeng/table';

import { ListShareImports } from '@/share/import/list-share-imports';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PrintTemplateModel } from '@/core/models/config/print-template.model';
import { PrintTemplateService } from '@/core/services/config/print-template/print-template.service';
import { PRINT_TEMPLATE_TYPE_OPTIONS } from '@/define/print-template.define';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-print-template-list',
  templateUrl: './print-template-list.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class PrintTemplateList extends BaseListV2<PrintTemplateModel> implements OnInit {
  @ViewChild('dt') dt!: Table<PrintTemplateModel>;
  modelService = inject(PrintTemplateService);
  private router = inject(Router);
  // Đặt tên khác `activatedRoute` — BaseListV2 đã khai `private activatedRoute` riêng, trùng tên
  // sẽ vỡ compile (TS2415: 2 khai báo private property khác class).
  private printTemplateRoute = inject(ActivatedRoute);
  protected override URL_STATE_PREFIX = 'pt';

  readonly typeOptions = PRINT_TEMPLATE_TYPE_OPTIONS;
  filterType = signal<string>('');

  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Tên template', labelKey: 'common.label.name', value: 'name' },
    { label: 'Loại phiếu', labelKey: 'config.nav.medical_form.template_type', value: 'template_type' },
    { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch_id' },
    { label: 'Mặc định', labelKey: 'common.label.default', value: 'is_default' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'is_active' },
    { label: 'Ngày tạo', labelKey: 'common.label.created_at', value: 'created_at' },
  ];

  override selectedColumns = signal(['name', 'template_type', 'branch_id', 'is_default', 'is_active']);

  ngOnInit(): void {
    // Loại phiếu (?type=...) lọc CỨNG theo route con trong menu — không phải filter người dùng
    // tự chỉnh trong panel, nên đi qua requiredSearch (luôn gộp vào searchParams), tách khỏi
    // filterFields()/_keyword.
    const type = this.printTemplateRoute.snapshot.queryParamMap.get('type') ?? '';
    if (type) {
      this.filterType.set(type);
      this.requiredSearch.set({ template_type: type });
    }
    this.initListV2();
  }

  /**
   * Khai báo filter — list này chỉ có ô tìm kiếm theo từ khoá.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  override createRecord(): void {
    const type = this.filterType();
    this.router.navigate(
      ['/config/print-template/create'],
      type ? { queryParams: { type } } : {},
    );
  }

  override updateRecord(model: PrintTemplateModel): void {
    this.router.navigate(['/config/print-template', model.id]);
  }
}
