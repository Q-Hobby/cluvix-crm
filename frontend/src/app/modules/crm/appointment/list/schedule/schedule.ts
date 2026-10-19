import {ChangeDetectionStrategy, ChangeDetectorRef, Component, inject, OnInit } from '@angular/core';
import { distinctUntilChanged, finalize, Subject, takeUntil } from 'rxjs';

import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseList } from '@/share/base/base-list';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ScheduleDetail } from '@/modules/crm/appointment/list/schedule-detail/schedule-detail';
import { debounceTime } from 'rxjs/operators';

// interfaces (tuỳ bạn để trong file riêng)
type Col = { key: string; label: string };
type Group = { groupLabel: string; branchId: number; cols: Col[] };
type Row = { day: string; cells: Record<string, number> };
type ApiRes = { columns: Group[]; rows: Row[]; types?: Array<{ value: number; label: string }> };
type TagSeverity = 'success' | 'info' | 'warn' | 'danger' | 'secondary' | 'contrast';

import { dequal } from '@/share/services/utils.service';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-schedule',
  templateUrl: './schedule.html',
  styles: [' '],
  standalone: true,
  imports: [...ListShareImports],
})
export class Schedule extends BaseList<any> implements OnInit {
  modelService = inject(AppointmentService);
  matrix: ApiRes | null = null;
  colW = 80; // px cho mỗi cột type
  dayColW = 120; // px cho cột "Ngày"
  private cdr = inject(ChangeDetectorRef);
  private branchColors = [
    { header: '#e0f2fe', cell: '#f0f9ff' }, // sky-100 / sky-50
    { header: '#fef3c7', cell: '#fffbeb' }, // amber-100 / amber-50
    { header: '#d1fae5', cell: '#ecfdf5' }, // emerald-100 / emerald-50
    { header: '#ede9fe', cell: '#f5f3ff' }, // violet-100 / violet-50
    { header: '#ffe4e6', cell: '#fff1f2' }, // rose-100 / rose-50
  ];
  private cancelPrevious$ = new Subject<void>();

  constructor() {
    super();
    this.padCols.set(6); // Số cột skeleton lúc loading — cột thật là pivot động, chưa biết trước.
  }

  // Phẳng cột + gắn kèm branchId để không phải parse key
  private _flatCols: Array<{
    key: string;
    label: string;
    branchId: number;
    first: boolean;
  }> | null = null;

  get flatCols(): Array<{ key: string; label: string; branchId: number; first: boolean }> {
    if (!this._flatCols && this.matrix?.columns) {
      this._flatCols = (this.matrix.columns ?? []).flatMap((g) =>
        g.cols.map((c, idx) => ({ ...c, branchId: g.branchId, first: idx === 0 })),
      );
    }
    return this._flatCols ?? [];
  }

  private _minWidth: string | null = null;

  get minWidth(): string {
    if (!this._minWidth && this.flatCols.length > 0) {
      const total = this.dayColW + this.flatCols.length * this.colW;
      this._minWidth = `${total}px`;
    }
    return this._minWidth ?? `${this.dayColW}px`;
  }

  ngOnInit() {
    queueMicrotask(() => this.searchHandler());
    this.initFilter();
    this.initBaseList();
  }

  /**
   * Filter lịch hẹn
   */
  initFilter() {
    this.rfFilterModel = this.fb.group({
      appointment_type_id: this.fb.control([]),
    });

    this.rfFilterModel.controls['appointment_type_id'].valueChanges
      .pipe(debounceTime(300), distinctUntilChanged(dequal), takeUntilDestroyed(this.destroyRef))
      .subscribe(() => {
        this.searchHandler();
      });
  }

  /**
   * Lấy danh sách thống kê
   */
  override searchHandler() {
    this.cancelPrevious$.next();
    this.matrix = null;
    this._flatCols = null; // Reset cache
    this._minWidth = null; // Reset cache
    this.loading.set(true);
    this.loadingService.showLoading('schedule');

    const typeIds = this.rfFilterModel.get('appointment_type_id')?.value;
    const options: any = {};
    if (typeIds && typeIds.length > 0) {
      options.params = { type_id: typeIds.join(',') };
    }

    this.modelService
      .getAppointmentScheduleByDate(options)
      .pipe(
        takeUntil(this.cancelPrevious$),
        takeUntilDestroyed(this.destroyRef),
        finalize(() => {
          this.loading.set(false);
          this.loadingService.hideLoading('schedule');
        }),
      )
      .subscribe((response: ApiRes) => {
        this.matrix = response;
        this.cdr.markForCheck();
      });
  }

  headerBg = (branchId: number) => this.branchColors[this.colorIdx(branchId)].header;

  isStatusCol(c: { key: string }) {
    return c.key.endsWith('_status');
  }

  // value = mã trạng thái BE trả ("normal"/"busy"/"very_busy"/"unknown"), KHÔNG phải text hiển thị.
  tagSeverityOf(value: string | null | undefined): TagSeverity {
    switch ((value || '').trim()) {
      case 'normal':
        return 'success';
      case 'busy':
        return 'warn'; // <— p-tag dùng 'warn'
      case 'very_busy':
        return 'danger';
      default:
        return 'secondary';
    }
  }

  busynessLabel(value: string | null | undefined): string {
    switch ((value || '').trim()) {
      case 'normal':
        return this.translate.instant('common.label.normal');
      case 'busy':
        return this.translate.instant('crm.appointment.label.busy');
      case 'very_busy':
        return this.translate.instant('crm.appointment.label.very_busy');
      default:
        return '—';
    }
  }

  onCellClick(day: string, branchId: number, cellKey: string, count: number) {
    if (!count || count <= 0 || this.isStatusCol({ key: cellKey })) return;

    // Parse appointment type from cellKey (format: b{branchId}_t{typeId})
    const typeId = cellKey.split('_t')[1];
    if (!typeId) return;

    this.loadAppointmentDetails(day, branchId, parseInt(typeId));
  }

  private loadAppointmentDetails(day: string, branchId: number, typeId: number) {
    this.dialogWrapperService.open(ScheduleDetail, {
      showHeader: false,
      width: '1024px',
      height: '80vh',
      contentStyle: { padding: '0' },
      data: {
        timeSlot: day,
        branchId: branchId,
        typeId: typeId,
        appointmentDate: this.parseDay(day),
        branch: this.getBranchName(branchId),
      },
    });
  }

  private getBranchName(branchId: number): string {
    const branch = this.matrix?.columns?.find((c) => c.branchId === branchId);
    return branch?.groupLabel || `${this.translate.instant('common.label.branch')} ${branchId}`;
  }

  private parseDay(day: string): Date {
    // Parse "10-10-2025" to Date
    const [d, m, y] = day.split('-').map(Number);
    return new Date(y, m - 1, d);
  }

  private colorIdx = (branchId: number) => (Math.max(1, branchId) - 1) % this.branchColors.length;
}
