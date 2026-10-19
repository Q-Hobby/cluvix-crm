import {ChangeDetectionStrategy, Component, computed, DestroyRef, inject, OnInit, signal} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {FormControl, ReactiveFormsModule} from '@angular/forms';
import {TranslatePipe, TranslateService} from '@ngx-translate/core';
import {ButtonModule} from 'primeng/button';
import {DatePickerModule} from 'primeng/datepicker';
import {FloatLabelModule} from 'primeng/floatlabel';
import {Skeleton} from 'primeng/skeleton';
import {TableModule} from 'primeng/table';
import {TooltipModule} from 'primeng/tooltip';
import {debounceTime, distinctUntilChanged, finalize} from 'rxjs';
import * as XLSX from 'xlsx-js-style';

import {TimezoneService} from '@/core/services/timezone.service';
import {PaymentByMethodRow, ReportRevenueCrmService} from '@/core/services/crm/report-revenue-crm.service';
import {PAYMENT_FOR, PAYMENT_FOR_OPTION} from '@/define/payment.define';
import {DatePickerFooter} from '@/share/components/date-picker-footer';
import {EmptyMessage} from '@/share/components/empty-message/empty-message';
import {CurrencyFormatPipe} from '@/share/pipes/currency-format/currency-format.pipe';
import {dequal} from '@/share/services/utils.service';
import {safeSheetName} from '@/share/utils/report-summary-export.util';

interface PivotRow {
  methodId: number;
  methodName: string;
  byPaymentFor: Partial<Record<number, number>>;
  total: number;
}

interface PaymentForCol {
  id: number;
  labelKey: string;
  label: string;
}

/** Loại phiếu KHÔNG phải dòng tiền thật — hiện cột riêng nhưng không cộng vào Tổng. */
export function isInternalTransfer(paymentFor: number): boolean {
  return paymentFor === PAYMENT_FOR.TRANSFER_DEPOSIT;
}

const PAYMENT_FOR_COL_MAP = new Map<number, PaymentForCol>(
  PAYMENT_FOR_OPTION.map((o) => [o.value, {id: o.value, labelKey: o.labelKey, label: o.label}]),
);

/**
 * Hình thức thanh toán — BE `GET crm/report/revenue/by-method` (port ERP accountant
 * `payment-method-report`). BE trả flat rows (hình thức × loại phiếu → tổng tiền), FE pivot thành
 * bảng chéo; copy/export Excel chạy client-side trên dữ liệu đã tải (không phân trang).
 * BE BẮT BUỘC from/to + trần 1 năm ⇒ chưa đủ 2 đầu ngày thì không gọi API.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-report-revenue-by-method',
  templateUrl: './revenue-by-method.html',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    TranslatePipe,
    DatePickerModule,
    ButtonModule,
    Skeleton,
    TooltipModule,
    FloatLabelModule,
    TableModule,
    CurrencyFormatPipe,
    DatePickerFooter,
    EmptyMessage,
  ],
  providers: [CurrencyFormatPipe],
})
export class RevenueByMethod implements OnInit {
  private readonly svc = inject(ReportRevenueCrmService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly tz = inject(TimezoneService);
  private readonly currency = inject(CurrencyFormatPipe);
  private readonly translate = inject(TranslateService);

  protected readonly loading = signal(false);
  protected readonly exporting = signal(false);
  protected readonly copied = signal(false);
  protected readonly rawRows = signal<PaymentByMethodRow[] | null>(null);

  protected readonly dateRangeCtrl = new FormControl<Date[]>([this.tz.monthStart(), this.tz.todayStart()]);

  protected readonly paymentForCols = computed<PaymentForCol[]>(() => {
    const rows = this.rawRows();
    if (!rows) return [];
    const seen = new Set<number>();
    for (const r of rows) seen.add(r.payment_for);
    return Array.from(seen)
      .sort((a, b) => a - b)
      .map((id) => PAYMENT_FOR_COL_MAP.get(id) ?? {id, labelKey: '', label: String(id)});
  });

  protected readonly pivot = computed<PivotRow[] | null>(() => {
    const rows = this.rawRows();
    if (rows === null) return null;
    const map = new Map<number, PivotRow>();
    for (const r of rows) {
      let entry = map.get(r.method_id);
      if (!entry) {
        entry = {methodId: r.method_id, methodName: r.method_name, byPaymentFor: {}, total: 0};
        map.set(r.method_id, entry);
      }
      entry.byPaymentFor[r.payment_for] = (entry.byPaymentFor[r.payment_for] ?? 0) + Number(r.amount);
      // Tổng = tiền thật vào/ra (phiếu hoàn/chi mang dấu âm từ BE). Chuyển cọc là bút toán nội bộ
      // cọc → dịch vụ: tiền đã tính một lần lúc nhận cọc, cộng nữa là đếm 2 lần (lens-money M-12).
      if (!isInternalTransfer(r.payment_for)) entry.total += Number(r.amount);
    }
    return Array.from(map.values()).sort((a, b) => b.total - a.total);
  });

  protected readonly grandTotal = computed(() => (this.pivot() ?? []).reduce((s, r) => s + r.total, 0));

  protected readonly isInternalTransfer = isInternalTransfer;

  protected colTotal(paymentForId: number): number {
    return (this.pivot() ?? []).reduce((s, r) => s + (r.byPaymentFor[paymentForId] ?? 0), 0);
  }

  /** Tên hình thức — BE trả rỗng khi hình thức đã bị xoá khỏi danh mục. */
  protected methodLabel(row: PivotRow): string {
    return row.methodName || this.translate.instant('crm.report.revenue.by_method.unknown_method');
  }

  ngOnInit() {
    this.load();
    this.dateRangeCtrl.valueChanges
      .pipe(debounceTime(300), distinctUntilChanged(dequal), takeUntilDestroyed(this.destroyRef))
      .subscribe((range) => {
        if (range?.[0] && range?.[1]) this.load();
      });
  }

  protected onQuickSelect({start, end}: {start: Date; end: Date}) {
    this.dateRangeCtrl.setValue([start, end]);
  }

  protected reload() {
    this.load();
  }

  protected clearFilter() {
    this.dateRangeCtrl.setValue([this.tz.monthStart(), this.tz.todayStart()], {emitEvent: false});
    this.load();
  }

  protected copyTable() {
    const rows = this.pivot();
    if (!rows?.length) return;
    const cols = this.paymentForCols();
    const sep = '─'.repeat(40);
    const header = [
      this.translate.instant('common.label.payment_method'),
      ...cols.map((c) => this.colLabel(c)),
      this.translate.instant('common.label.total'),
    ].join(' | ');
    const lines = [
      this.translate.instant('crm.report.revenue.by_method.title'),
      sep,
      header,
      sep,
      ...rows.map((r) => {
        const cells = cols.map((c) => this.currency.transform(r.byPaymentFor[c.id] ?? 0));
        return [this.methodLabel(r), ...cells, this.currency.transform(r.total)].join(' | ');
      }),
    ];
    if (rows.length > 1) {
      const totals = cols.map((c) => this.currency.transform(this.colTotal(c.id)));
      lines.push(
        sep,
        [this.translate.instant('common.label.total'), ...totals, this.currency.transform(this.grandTotal())].join(' | '),
      );
    }
    navigator.clipboard.writeText(lines.join('\n')).then(() => {
      this.copied.set(true);
      setTimeout(() => this.copied.set(false), 2000);
    });
  }

  protected triggerExport() {
    if (this.exporting()) return;
    this.exporting.set(true);
    setTimeout(() => {
      this.exportXlsx();
      this.exporting.set(false);
    }, 0);
  }

  private colLabel(c: PaymentForCol): string {
    return c.labelKey ? this.translate.instant(c.labelKey) : c.label;
  }

  private exportXlsx() {
    const rows = this.pivot();
    if (!rows?.length) return;
    const cols = this.paymentForCols();
    const range = this.dateRangeCtrl.value;
    const fmt = (d: Date) => this.tz.format(d, {day: '2-digit', month: '2-digit', year: 'numeric'}).replace(/\//g, '-');
    const slug = range?.[0] ? `${fmt(range[0])}_${fmt(range[1] ?? range[0])}` : 'export';
    const title = this.translate.instant('crm.report.revenue.by_method.title');
    const headers = [
      this.translate.instant('common.label.payment_method'),
      ...cols.map((c) => this.colLabel(c)),
      this.translate.instant('common.label.total'),
    ];
    const dataRows: (string | number)[][] = rows.map((r) => [
      this.methodLabel(r),
      ...cols.map((c) => r.byPaymentFor[c.id] ?? 0),
      r.total,
    ]);
    if (rows.length > 1) {
      dataRows.push([
        this.translate.instant('common.label.total'),
        ...cols.map((c) => this.colTotal(c.id)),
        this.grandTotal(),
      ]);
    }
    const ws = XLSX.utils.aoa_to_sheet([[title], [], headers, ...dataRows]);
    ws['!cols'] = [{wch: 28}, ...cols.map(() => ({wch: 18})), {wch: 18}];
    ws['!merges'] = [{s: {r: 0, c: 0}, e: {r: 0, c: headers.length - 1}}];
    headers.forEach((_, i) => {
      const addr = XLSX.utils.encode_cell({r: 2, c: i});
      if (ws[addr]) ws[addr].s = {font: {bold: true}};
    });
    const wb = XLSX.utils.book_new();
    // Tên sheet tối đa 31 ký tự — dùng key ngắn riêng.
    XLSX.utils.book_append_sheet(wb, ws, safeSheetName(this.translate.instant('crm.report.revenue.by_method.sheet_name')));
    XLSX.writeFile(wb, `hinh-thuc-thanh-toan-${slug}.xlsx`);
  }

  private load() {
    const range = this.dateRangeCtrl.value;
    if (!range?.[0] || !range?.[1]) return;
    const from = this.tz.dateHmsToUnix(range[0], 0, 0, 0);
    const to = this.tz.dateHmsToUnix(range[1], 23, 59, 59);
    this.loading.set(true);
    this.rawRows.set(null);
    this.svc
      .getPaymentByMethod(from, to)
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (res) => this.rawRows.set(res ?? []),
        error: () => this.rawRows.set([]),
      });
  }
}
