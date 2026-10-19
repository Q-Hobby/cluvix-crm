import { Directive, OnDestroy, signal, ViewChild } from '@angular/core';
import { FormGroup } from '@angular/forms';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';
import { Table } from 'primeng/table';

import { BaseApp } from '@/share/base/BaseApp';
import { perPageOptionsTable, rowsTable, STORAGE_KEY_SELECT } from '@/define';

/**
 * Base class cho tất cả báo cáo dược phẩm.
 * Xử lý: pagination, keyword search, date range, export, loading state.
 *
 * Class con chỉ cần:
 *  - Khai báo rfFilter
 *  - Implement fetchData() và fetchExport()
 *  - Override transformItems() nếu cần addLabels
 */
@Directive()
export abstract class BaseReport<T = any> extends BaseApp implements OnDestroy {
  override loading = signal(false);
  timer: any = null;

  models = signal<T[]>([]);
  paginationInfo = signal<any>(null);
  totals = signal<any>({});
  page = signal(1);
  perPage = signal(rowsTable);
  perPageOptions = perPageOptionsTable;

  abstract rfFilter: FormGroup;

  constructor() {
    super();
    const saved = this.storageService.loadStorage<string>(STORAGE_KEY_SELECT.KEY_ROWS_TABLE, false);
    if (saved) this.perPage.set(parseInt(saved));
  }

  protected abstract fetchData(): Observable<any>;
  protected abstract fetchExport(): Observable<string>;

  protected transformItems(items: any[]): Promise<T[]> | T[] {
    return items as T[];
  }

  // ─── Helpers ───

  protected get keyword(): string | undefined {
    return (this.rfFilter.get('_keyword')?.value || '').trim() || undefined;
  }

  protected get branchIds(): number[] {
    return this.rfFilter.get('branch_id')?.value ?? [];
  }

  protected getDateRange(): { from: number; to: number } | null {
    const dr = this.rfFilter.get('dateRange')?.value as Date[] | null;
    if (!dr?.[0]) return null;
    const from = this.timezoneService.dateHmsToUnix(dr[0], 0, 0, 0);
    const to = dr[1] ? this.timezoneService.dateHmsToUnix(dr[1], 23, 59, 59) : this.timezoneService.todayEndUnix();
    return { from, to };
  }

  // ─── Core ───

  loadReport() {
    this.loading.set(true);
    this.fetchData().subscribe({
      next: async (res: any) => {
        try {
          this.models.set(await this.transformItems(res.items ?? []));
          this.paginationInfo.set(res._meta ?? null);
          this.totals.set(res._totals ?? {});
        } catch { this.models.set([]); }
        finally { this.loading.set(false); }
      },
      error: () => { this.models.set([]); this.loading.set(false); },
    });
  }

  onPageChange(event: any) {
    this.page.set(Math.floor(event.first / event.rows) + 1);
    this.perPage.set(event.rows);
    if (typeof event.rows === 'number') {
      this.storageService.saveStorage(STORAGE_KEY_SELECT.KEY_ROWS_TABLE, event.rows, true);
    }
    this.loadReport();
  }

  searchKeyword(event: any) {
    if (this.timer) clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      this.rfFilter.patchValue({ _keyword: event.target.value }, { emitEvent: false });
      this.page.set(1);
      this.loadReport();
    }, 500);
  }

  onFilterChange() {
    this.page.set(1);
    if (this.dt) this.dt.first.set(0);
    this.loadReport();
  }

  onDateSelect() {
    const dr = this.rfFilter.get('dateRange')?.value as Date[] | null;
    if (dr?.[0] && dr?.[1]) this.onFilterChange();
  }

  onQuickSelect({ start, end }: { start: Date; end: Date }) {
    this.rfFilter.patchValue({ dateRange: [start, end] }, { emitEvent: false });
    this.onFilterChange();
  }

  exportExcel() {
    this.loadingService.showLoading('exportReport');
    this.fetchExport()
      .pipe(finalize(() => this.loadingService.hideLoading('exportReport')))
      .subscribe({
        next: (url: string) => {
          this.noti.successMsg(this.translate.instant('common.message.file_downloading'));
          this.downloadExport(url);
        },
      });
  }

  resetPage() {
    this.models.set([]);
    this.totals.set({});
    this.page.set(1);
  }

  @ViewChild(Table) protected dt!: Table;

  ngOnDestroy() {
    if (this.timer) { clearTimeout(this.timer); this.timer = null; }
  }
}
