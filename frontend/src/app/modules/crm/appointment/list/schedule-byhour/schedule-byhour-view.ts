import {ChangeDetectionStrategy, ChangeDetectorRef,
  Component,
  inject,
  Input,
  OnChanges,
  OnInit,
  SimpleChanges,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize, Subject, takeUntil } from 'rxjs';

import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseList } from '@/share/base/base-list';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ScheduleDetail } from '@/modules/crm/appointment/list/schedule-detail/schedule-detail';

// interfaces (tuỳ bạn để trong file riêng)
type Col = { key: string; label: string };
type Group = { groupLabel: string; branchId: number; cols: Col[] };
type Row = { timeSlot: string; cells: Record<string, number> };
type ApiRes = { columns: Group[]; rows: Row[] };
type TagSeverity = 'success' | 'info' | 'warn' | 'danger' | 'secondary' | 'contrast';

import { dequal } from '@/share/services/utils.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-schedule-byhour-view',
  templateUrl: './schedule-byhour-view.html',
  styles: [' '],
  standalone: true,
  imports: [...ListShareImports],
})
export class ScheduleByhourView extends BaseList<any> implements OnInit, OnChanges {
  @Input() appointmentDate?: Date;

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
  private _minWidth: string | null = null;
  private cancelPrevious$ = new Subject<void>();

  constructor() {
    super();
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

  ngOnInit() {
    queueMicrotask(() => this.searchHandler());
    this.initBaseList();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['appointmentDate'] && !changes['appointmentDate'].firstChange) {
      this.searchHandler();
    }
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
    const date: Date = this.appointmentDate ?? this.timezoneService.now();
    const params = { appointment_date: this.toYmd(date) };
    this.modelService
      .getAppointmentScheduleByHour({ params })
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

  calcMinWidth(): string {
    if (!this._minWidth && this.flatCols.length > 0) {
      const total = this.dayColW + this.flatCols.length * this.colW;
      this._minWidth = `${total}px`;
    }
    return this._minWidth ?? '100%';
  }

  isStatusCol(c: { key: string }) {
    return c.key.endsWith('_status');
  }

  tagSeverityOf(value: string | null | undefined): TagSeverity {
    switch ((value || '').trim()) {
      case 'Bình thường':
        return 'success';
      case 'Đông':
        return 'warn'; // <— p-tag dùng 'warn'
      case 'Rất đông':
        return 'danger';
      default:
        return 'secondary';
    }
  }

  onCellClick(timeSlot: string, branchId: number, cellKey: string, count: number) {
    if (!count || count <= 0 || this.isStatusCol({ key: cellKey })) return;

    // Parse appointment type from cellKey (format: b{branchId}_t{typeId})
    const typeId = cellKey.split('_t')[1];
    if (!typeId) return;

    this.loadAppointmentDetails(timeSlot, branchId, parseInt(typeId));
  }

  private toYmd(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  private colorIdx = (branchId: number) => (Math.max(1, branchId) - 1) % this.branchColors.length;

  private loadAppointmentDetails(timeSlot: string, branchId: number, typeId: number) {
    this.dialogWrapperService.open(ScheduleDetail, {
      showHeader: false,
      width: '960px',
      height: '80vh',
      contentStyle: { padding: '0' },
      data: {
        timeSlot: timeSlot,
        branchId: branchId,
        typeId: typeId,
        appointmentDate: this.appointmentDate,
        branch: this.getBranchName(branchId),
      },
    });
  }

  private getBranchName(branchId: number): string {
    const branch = this.matrix?.columns?.find((c) => c.branchId === branchId);
    return branch?.groupLabel || `${this.translate.instant('common.label.branch')} ${branchId}`;
  }
}
