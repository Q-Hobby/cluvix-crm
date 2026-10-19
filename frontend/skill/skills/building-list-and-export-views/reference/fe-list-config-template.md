# Khuôn list config mới (BaseListV2)

## Mục lục

- Mẫu tham chiếu
- `.ts`
- `.html`
- Không chép

---

## Mẫu tham chiếu

`src/app/modules/config/config-medicine/med-dqg-doctor/list/` — list routed dưới `/config`, có từ khoá + 1 ô chọn, bộ chọn cột, cột thao tác cố định. Tỷ lệ pattern đo trên 105 list `modules/config/*/*/list/*-list.html`.

## `.ts`

```ts
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-xxx-list',
  templateUrl: './xxx-list.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class XxxList extends BaseListV2<XxxModel> implements OnInit {
  modelService = inject(XxxService);
  @ViewChild('dt') dt!: Table<XxxModel>;
  protected override URL_STATE_PREFIX = 'xx';

  // createRecord()/updateRecord()/deleteRecord() của base mở `formComponent` và tự check 2 quyền này
  override formComponent = XxxForm;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;

  readonly statusOptions = [
    { value: 1, label: this.translate.instant('common.label.active') },
    { value: 0, label: this.translate.instant('common.label.inactive') },
  ];

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }

  displayCols = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Tên', labelKey: 'common.label.name', value: 'name' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
  ];
  override selectedColumns = signal(['id', 'name', 'status']);

  constructor() {
    super();
    // + 2 = cột STT `#` + cột thao tác (luôn hiện)
    effect(() => this.padCols.set((this.selectedColumns()?.length ?? 0) + 2));
  }

  ngOnInit() { this.initListV2(); }

  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }, { name: 'status', kind: 'equals' }];
  }
}
```

## `.html`

```html
<p-table #dt (onLazyLoad)="loadRecords($event)" [columns]="cols()" [filterDelay]="0" [filters]="initialFilters()"
  [first]="first()" [lazy]="true" [lazyLoadOnInit]="false" [loading]="showSkeleton()" [paginator]="true"
  [rowHover]="true" [rowsPerPageOptions]="isDesktop() ? perPageOptionsTable : []" [rows]="perPage()"
  [scrollHeight]="'flex'" [scrollable]="true" [showCurrentPageReport]="true" [showLoader]="false"
  [stripedRows]="true" [style]="{ width: '100%' }" appFillHeight="page"
  [totalRecords]="paginationInfo()?.totalCount ?? 0" [value]="models()"
  [currentPageReportTemplate]="isDesktop() ? ('common.page.current_report' | translate) : ''" dataKey="id">
  <ng-template #caption>
    <div class="h-[2px] mb-1">
      @if (refreshing()) { <p-progressbar [showValue]="false" [style]="{ height: '2px' }" mode="indeterminate" /> }
    </div>
    <div class="flex items-center gap-2 justify-between">
      <div class="p-toolbar-group-start">
        <button type="button" pButton (click)="createRecord()" *hasPerm="permission.permissionManageConfig"
          [attr.aria-label]="'common.action.create' | translate">
          <i class="pi pi-plus"></i><span>{{ isDesktop() ? ('common.action.create' | translate) : '' }}</span>
        </button>
      </div>
      <div class="p-toolbar-group-end">
        @if (rfFilterModel) {
          <form [formGroup]="rfFilterModel" novalidate>
            <div class="flex items-center gap-2 w-full">
              @if (isDesktop()) {
                <div class="flex-grow">
                  <p-floatlabel variant="on">
                    <p-iconfield>
                      <p-inputicon class="pi pi-search" />
                      <input autocomplete="off" formControlName="_keyword" id="search_keyword" pInputText type="text" />
                    </p-iconfield>
                    <label for="search_keyword">{{ 'common.label.keyword' | translate }}</label>
                  </p-floatlabel>
                </div>
                <p-floatlabel variant="on" class="flex-1 min-w-[140px]">
                  <p-select [options]="statusOptions" [style]="{ minWidth: '100%' }" appendTo="body" class="w-full"
                    [showClear]="true" formControlName="status" id="status" optionLabel="label" optionValue="value" />
                  <label for="status">{{ 'common.label.status' | translate }}</label>
                </p-floatlabel>
              }
              <button pButton type="button" [iconOnly]="true" (click)="colsTpl.toggle($event)" tooltipPosition="top"
                [attr.aria-label]="'common.action.view_more' | translate" [pTooltip]="'common.action.view_more' | translate">
                <i class="pi pi-th-large"></i>
              </button>
            </div>
          </form>
        }
      </div>
    </div>
  </ng-template>

  <ng-template #header>
    <tr>
      <th class="w-[30px] min-w-[30px]"><div class="flex items-center justify-center w-full">#</div></th>
      @if (isColVisible('id')()) {
        <th class="w-[30px] min-w-[30px]"><div class="flex items-center justify-center w-full">ID</div></th>
      }
      @if (isColVisible('name')()) { <th class="min-w-[180px]">{{ 'common.label.name' | translate }}</th> }
      @if (isColVisible('status')()) { <th class="min-w-[100px] w-[100px]">{{ 'common.label.status' | translate }}</th> }
      <th [frozen]="isDesktop()" alignFrozen="right" class="min-w-[80px] w-[80px]" pFrozenColumn>
        <div class="flex items-center justify-center w-full">#</div>
      </th>
    </tr>
  </ng-template>

  <ng-template #body let-index="rowIndex" let-model>
    <tr [class.tr-disabled]="!model.status">
      <td><div class="flex items-center justify-center w-full">{{ index + 1 }}</div></td>
      @if (isColVisible('id')()) { <td><div class="flex items-center justify-center w-full">{{ model.id }}</div></td> }
      @if (isColVisible('name')()) {
        <td [appCopyText]="model.name"><span class="font-semibold text-primary">{{ model.name }}</span></td>
      }
      @if (isColVisible('status')()) {
        <td>
          <p-toggleswitch (onChange)="handleStatusChange(model.id)" [(ngModel)]="model.status"
            [disabled]="checkUpdatePermission()" [trueValue]="1" [falseValue]="0" />
        </td>
      }
      <td [frozen]="isDesktop()" alignFrozen="right" pFrozenColumn>
        <div class="cx-row-actions">
          <p-buttongroup>
            <p-button icon="pi pi-pencil" severity="success" size="small" [iconOnly]="true" tooltipPosition="top"
              (onClick)="updateRecord(model)" *hasPerm="permission.permissionManageConfig"
              [ariaLabel]="'common.action.edit' | translate" [pTooltip]="'common.action.update' | translate" />
            <p-button icon="pi pi-trash" severity="danger" size="small" [iconOnly]="true" tooltipPosition="top"
              (onClick)="deleteRecord($event, model.id)" *hasPerm="permission.permissionManageConfig"
              [ariaLabel]="'common.action.delete' | translate" [pTooltip]="'common.action.delete' | translate" />
          </p-buttongroup>
        </div>
      </td>
    </tr>
  </ng-template>

  <ng-template #loadingbody>
    @for (_ of rowsIt(); track $index) {
      <tr class="h-[46px]">
        @for (_ of colsIt(); track $index) { <td><p-skeleton></p-skeleton></td> }
      </tr>
    }
  </ng-template>

  <ng-template #emptymessage>
    <tr><td [attr.colspan]="selectedColumns().length + 10"><app-empty-message></app-empty-message></td></tr>
  </ng-template>
</p-table>
<p-popover #colsTpl>
  <app-column-toggle (changeToggle)="selectedColumns.set($event); saveSelectedColumns()"
    [columns]="displayCols" [selectedCols]="selectedColumns" />
</p-popover>
```

- Vùng lọc bọc `@if (isDesktop())` (96/105); nút chọn cột nằm NGOÀI `@if`.
- Ô chọn đặt SAU ô từ khoá, bọc `p-floatlabel variant="on" class="flex-1 min-w-[140px]"`.
- Mọi import trên đã có trong `ListShareImports`; `FillHeightDirective` import riêng (xem `appFillHeight="page"` ở `SKILL.md`).

## Không chép

- `class="vertical-align-top"` trên `<tr>` — không có CSS nào định nghĩa, không có tác dụng.
