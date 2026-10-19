import { computed, DestroyRef, Directive, inject, OnDestroy, signal, Type } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormGroup } from '@angular/forms';
import { FileSelectEvent } from 'primeng/fileupload';
import { finalize } from 'rxjs';

import { Identifiable } from '@/core/interface/identifiable';
import { BaseApp } from '@/share/base/BaseApp';
import {

  PERMISSION_SELECT,
  perPageOptionsTable,
  rowsTable,
  STORAGE_KEY_SELECT,
} from '@/define';
import { ListApiResponse, PaginationInfo } from '@/share/models/list-api-response';
import { BaseApiService } from '@/share/services/base-api.service';
import { UtilsService } from '@/share/services/utils.service';

@Directive()
export abstract class BaseList<T extends Identifiable> extends BaseApp implements OnDestroy {
  permission = PERMISSION_SELECT;
  abstract modelService: BaseApiService<T>;

  timer: any = null;
  fileToUpload: File | null = null;
  formComponent!: Type<any>;
  updatePermissionString = '';
  updateDialogTitleString = '';
  updateDialogTitleKey = '';
  createPermissionString = '';
  createDialogTitleString = '';
  createDialogTitleKey = '';
  perPageOptionsTable = perPageOptionsTable;
  readonly page = signal(1);
  readonly perPage = signal(rowsTable);
  readonly sort = signal('-id');
  readonly models = signal<T[]>([]);
  readonly paginationInfo = signal<PaginationInfo | null>(null);
  readonly selectedRecords = signal<T[]>([]);
  readonly cols = signal<any[]>([]);
  readonly search = signal<Partial<Record<string, any>>>({});
  readonly defaultSearch = signal<Partial<Record<string, any>>>({});
  readonly requiredSearch = signal<Partial<Record<string, any>>>({});
  selectedColumns = signal<string[]>([]);
  readonly searchParams = computed(() => ({
    page: this.page(),
    sort: this.sort(),
    'per-page': this.perPage(),
    ...this.defaultSearch(),
    ...this.search(),
    ...this.requiredSearch(),
  }));
  readonly skeletonRows = signal(this.perPage());
  readonly padCols = signal(0);
  protected utilsService = inject(UtilsService);
  protected destroyRef = inject(DestroyRef);
  protected LS_SELECTED_COLUMNS_KEY = '';
  protected STORAGE_KEY_FILTER = '';
  protected STORAGE_KEY_SORT = '';
  protected rfFilterModel!: FormGroup;
  protected isOpeningForm = signal(false);
  /**
   *
   * @private
   */
  protected prevCellValues = new Map<string, any>();

  constructor() {
    super();
    const saved = this.storageService.loadStorage<string>(STORAGE_KEY_SELECT.KEY_ROWS_TABLE, false);
    if (saved) this.perPage.set(parseInt(saved));
  }

  /**
   * Kiểm tra cột có hiển thị không
   */
  isColVisible = (col: string) =>
    computed(() => Array.isArray(this.selectedColumns()) && this.selectedColumns().includes(col));

  /**
   * Gọi API lấy danh sách dữ liệu theo searchParams
   */
  searchHandler() {
    this.modelService
      .list({ params: this.searchParams() })
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.loading.set(false)),
      )
      .subscribe({
        next: (response: ListApiResponse<T>) => {
          this.models.set(response.items ?? []);
          this.paginationInfo.set(response._meta ?? null);
        },
        error: (err: any) => {
          this.models.set([]);
          console.error(err);
        },
      });
  }

  /**
   * Reset danh sách và gọi lại searchHandler
   */
  getList() {
    this.loading.set(true);
    this.models.set([]);
    this.searchHandler();
  }

  /**
   * Xử lý sự kiện load dữ liệu khi thay đổi phân trang, sort, filter
   */
  loadRecords(event: any) {
    const { first, rows, sortField, sortOrder, filters } = event;
    const page = Math.floor(first / rows) + 1;
    const filter = this.utilsService.buildFilter(filters);
    const sort = sortField ? (sortOrder === 1 ? sortField : `-${sortField}`) : this.sort();
    this.page.set(page);
    this.perPage.set(rows);
    if (typeof rows === 'number') {
      this.storageService.saveStorage(STORAGE_KEY_SELECT.KEY_ROWS_TABLE, rows, true);
    }
    this.sort.set(sort);
    this.saveSort();
    this.search.set(filter);
    this.saveFilter();
    this.getList();
  }

  /**
   * Xử lý upload file import dữ liệu
   */
  onBasicUploadAuto(files: FileSelectEvent, form: any) {
    this.fileToUpload = files['files'][0];
    // Template chuẩn: import-button ngay trước <p-fileupload class="hidden">.
    // Anchor confirmpopup vào button (visible) — fileupload host hidden → popup render ở 0,0.
    const host: HTMLElement = form.el.nativeElement;
    const anchor = (host.previousElementSibling as HTMLElement) ?? host;
    this.confirmationService.confirm({
      target: anchor,
      message: this.translate.instant('common.message.import_confirm'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.modelService
          .importData(this.fileToUpload!)
          .pipe(finalize(() => form.clear()))
          .subscribe({
            next: () => {
              this.noti.successMsg(this.translate.instant('common.message.import_success'));
              this.getList();
            },
          });
      },
      reject: () => {
        this.getList();
        form.clear();
      },
    });
  }

  /**
   * Xử lý thay đổi trạng thái (ẩn/hiện) của bản ghi
   */
  handleStatusChange(id: number) {
    this.toggleField(id, 'status');
  }

  /**
   * Toggle 1 field boolean (0/1) qua p-toggleswitch dùng chung cho mọi list.
   * [(ngModel)] đã đổi giá trị field TRƯỚC khi (onChange) fire — gọi API lưu ngay, và
   * rollback về giá trị cũ nếu lỗi (vd version conflict) để UI không nói đã lưu trong khi
   * DB chưa đổi (silent mismatch nếu thiếu bước rollback này).
   */
  protected toggleField(id: number, field: string) {
    const row = this.models().find((m: any) => m.id === id) as any;
    if (!row) return;
    const newVal = +row[field] ? 1 : 0;
    const prevVal = newVal ? 0 : 1;
    this.modelService.update(id, { [field]: newVal, version: row.version } as any).subscribe({
      next: (res: any) => {
        const updated = res?.data ?? res;
        const existing = this.models().find((m: any) => m.id === updated.id);
        if (existing) {
          this.updateRow({ ...existing, [field]: updated[field], version: updated.version } as T);
        }
        this.noti.successRes(res);
      },
      error: (err: any) => {
        const existing = this.models().find((m: any) => m.id === id);
        if (existing) {
          this.updateRow({ ...existing, [field]: prevVal } as T);
        }
        this.noti.errorMsg(err?.error?.message || this.translate.instant('common.message.error'));
      },
    });
  }

  /**
   * Xử lý lỗi hình ảnh (avatar)
   */
  errorHandler(event: any) {
    event.target.src = 'assets/overrides/images/no-avatar.svg';
  }


  /**
   * Tìm label theo value trong mảng object
   */
  findValue(idValue: any, objectArray: { value: any; label: string }[]): string | undefined {
    return objectArray.find((x) => x.value === idValue)?.label;
  }

  /**
   * Mở dialog tạo mới bản ghi
   */
  createRecord(): void {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.createDialogTitleKey
      ? this.translate.instant(this.createDialogTitleKey)
      : (this.createDialogTitleString || this.translate.instant('common.action.create'));
    const dialogRef = this.dialogWrapperService.open(this.formComponent, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: T) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  updateRecord(model: T) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.updateDialogTitleKey
      ? this.translate.instant(this.updateDialogTitleKey)
      : (this.updateDialogTitleString || this.translate.instant('common.action.update'));
    const dialogRef = this.dialogWrapperService.open(this.formComponent, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: T) => {
      if (result) {
        this.updateRow(result);
      }
    });
  }

  /**
   * Cập nhật lại dữ liệu trong models khi có bản ghi mới
   */
  updateRow(updated: T): void {
    const current = this.models();
    const index = current.findIndex((item) => item.id === updated.id);
    if (index !== -1) {
      const cloned = [...current];
      cloned[index] = updated;
      this.models.set(cloned);
    }
  }

  /**
   * Xóa bản ghi
   */
  deleteRecord(event: Event, id: number): void {
    this.isSubmitting.set(true);
    this.confirmationService.confirm({
      target: event.currentTarget as EventTarget,
      message: this.translate.instant('common.label.do_you_want_to_delete'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.loadingService.showLoading('deleteRecord');
        this.modelService
          .delete(id)
          .pipe(
            finalize(() => {
              this.isSubmitting.set(false);
              this.loadingService.hideLoading('deleteRecord');
            }),
          )
          .subscribe({
            next: (res: any) => {
              this.getList();
              // BE trả {message: "Xóa thành công"} — dùng successRes để đọc res.message
              this.noti.successRes(res);
            },
          });
      },
      reject: () => {
        this.isSubmitting.set(false);
      },
    });
  }

  /**
   * Lưu lại các cột đã chọn vào localStorage
   */
  saveSelectedColumns() {
    if (this.LS_SELECTED_COLUMNS_KEY) {
      this.storageService.saveStorage(this.LS_SELECTED_COLUMNS_KEY, this.selectedColumns());
    }
  }

  /**
   * Load các cột đã chọn từ localStorage
   */
  loadSelectedColumns() {
    const loaded = this.storageService.loadStorage<string[]>(this.LS_SELECTED_COLUMNS_KEY, true);
    this.selectedColumns.set(Array.isArray(loaded) ? loaded : this.selectedColumns());
  }

  /**
   * Lưu filter vào localStorage
   */
  saveFilter() {
    if (this.STORAGE_KEY_FILTER && this.search()) {
      this.storageService.saveStorage(this.STORAGE_KEY_FILTER, this.search());
    }
  }

  /**
   * Load filter từ localStorage
   */
  loadFilter() {
    if (this.STORAGE_KEY_FILTER) {
      const filter = this.storageService.loadStorage<any>(this.STORAGE_KEY_FILTER);
      if (filter) {
        this.search.set(filter);
      }
    }
  }

  /**
   * Lưu sort vào localStorage
   */
  saveSort() {
    if (this.STORAGE_KEY_SORT && this.sort()) {
      this.storageService.saveStorage(this.STORAGE_KEY_SORT, this.sort());
    }
  }

  /**
   * Load sort từ localStorage
   */
  loadSort() {
    if (this.STORAGE_KEY_SORT) {
      const sort = this.storageService.loadStorage<string>(this.STORAGE_KEY_SORT, false);
      if (sort) {
        this.sort.set(sort);
      }
    }
  }

  /**
   * Xử lý thay đổi lựa chọn bản ghi
   */
  onSelectionChange(val: T[]) {
    this.selectedRecords.set(val);
  }

  /**
   * Hàm hủy, đóng dialog khi destroy component
   */
  ngOnDestroy(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    if (this.dialogRef) {
      this.dialogRef.close(false);
    }
  }

  /**
   * Reset lại trang, gọi lại getList
   */
  resetPage() {
    this.getList();
  }

  /**
   * Khởi tạo giá trị trước đó
   * @param e
   */
  onCellEditInit(e: any) {
    const rowId = e.data?.id ?? e.index?.id;
    const field = e.field ?? 'SortOrder';
    const key = `${rowId}:${field}`;
    this.prevCellValues.set(key, e.data?.[field]);
  }

  /**
   * Xử lý sự kiện khi chỉnh sửa ô trong table
   * @param e
   */
  onCellEditComplete(e: any) {
    const id = e.data?.id ?? e.index?.id;
    const field = e.field ?? 'SortOrder';
    const key = `${id}:${field}`;
    const prev = this.prevCellValues.get(key);
    const curr = e.data?.[field];

    // Không đổi thì thôi
    if (String(prev) === String(curr)) {
      this.prevCellValues.delete(key);
      return;
    }

    const patch: any = { [field]: curr, version: e.data?.version };

    this.modelService.update(id, patch).subscribe({
      next: (res: any) => {
        const updated = res?.data ?? res;
        // Merge thay vì replace nguyên row — updated là entity thô từ BE, không có các field
        // label FE tự gắn (addLabel/addLabels qua IndexedDB helper). Replace nguyên row sẽ làm
        // mất các label đó cho tới khi reload (vd companyInfo/unitLabel ở ServiceList).
        const existing = this.models().find((m: any) => m.id === updated?.id) as any;
        this.updateRow({ ...(existing ?? {}), ...updated } as T);
        const messageKey = field === 'SortOrder' ? 'common.message.saved_position' : 'common.message.saved_value';
        this.noti.success(this.translate.instant(messageKey, { prev, curr }));
        this.prevCellValues.delete(key);
      },
      error: (err) => {
        if (e.data) this.updateRow({ ...e.data, [field]: prev } as T);
        this.noti.error(this.translate.instant('common.message.save_error'), err?.message || '');
        this.prevCellValues.delete(key);
      },
    });
  }

  /**
   * Hàm khởi tạo base, gọi trong ngOnInit của component con
   */
  protected initBaseList() {
    this.loadFilter();
    this.loadSort();
    this.afterBaseListInit();
    this.loadSelectedColumns();
  }

  /**
   * Hàm để override ở class con nếu cần
   */
  protected afterBaseListInit(): void {
    // Optional override in children
  }

  /**
   * Tạo mảng số lượng n phần tử
   */
  private makeRange = (n: number) => Array.from({ length: Math.max(0, n | 0) });

  /**
   * Mảng số lượng dòng skeleton
   */
  readonly rowsIt = computed(() => this.makeRange(this.skeletonRows()));
  /**
   * Mảng số lượng cột padding
   */
  readonly colsIt = computed(() => this.makeRange(this.padCols()));
}
