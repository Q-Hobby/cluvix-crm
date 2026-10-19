import * as XLSX from 'xlsx-js-style';
import {TranslateService} from '@ngx-translate/core';

import {NotificationService} from '@/share/services/notification.service';
import {PermissionsService} from '@/share/services/permissions.service';

/**
 * Helper copy/export cho các báo cáo doanh thu dạng tổng hợp (report-revenue/by-*).
 *
 * Dữ liệu đã tải hết ở FE nên copy/export chạy client-side, KHÔNG gọi BE.
 * Mọi báo cáo Nhóm A (by-branch, by-source, by-country, ...) có cùng cấu trúc:
 * cột nhãn đầu (item[labelField]) + các cột số (cols) + dòng Tổng (totals).
 *
 * Usage trong component:
 * ```typescript
 * import {copyReportSummary, exportReportSummary} from '@/share/utils/report-summary-export.util';
 *
 * copyData() {
 *   copyReportSummary(this.buildReportCfg(), this);
 * }
 * exportSummary() {
 *   exportReportSummary(this.buildReportCfg(), this);
 * }
 * ```
 *
 * `this` (BaseList) đã có sẵn translate/noti/permissionsService nên truyền thẳng làm deps.
 */

export interface ReportSummaryCol {
  /** tên field trong item / totals, vd 'totalRevenue' */
  value: string;
  /** i18n key cho tiêu đề cột, vd 'common.label.revenue' */
  labelKey: string;
}

/** Services lấy từ BaseList — truyền tường minh vì helper chạy ngoài injection context. */
export interface ReportSummaryDeps {
  translate: TranslateService;
  noti: NotificationService;
  permissionsService: PermissionsService;
}

export interface ReportSummaryConfig {
  /** Danh sách dòng đã map (this.models()). */
  items: any[];
  /** Các cột số đang hiển thị (displayCols đã lọc theo selectedColumns). */
  cols: ReportSummaryCol[];
  /** Dòng tổng (this.modelsTotal()). */
  totals: Record<string, any>;
  /** i18n key tiêu đề cột đầu, vd 'common.label.branch'. */
  firstColHeaderKey: string;
  /** i18n key tiêu đề báo cáo, vd 'accountant.report.revenue_by_branch'. */
  titleKey: string;
  /** Tên file export (không đuôi), vd 'doanh-thu-theo-chi-nhanh'. */
  filenameBase: string;
  /** Quyền yêu cầu, vd permission.permissionExtPayment. */
  permission: string;
  /** Field chứa nhãn dòng trong item. Mặc định 'name'. */
  labelField?: string;
  /** Format nhãn cột đầu (vd unix → ngày). Mặc định lấy nguyên item[labelField]. */
  labelFormatter?: (value: any, item: any) => string;
  /** Khoảng ngày lọc (rfFilterModel created_at) để đặt tên file. */
  dateRange?: (Date | null | undefined)[] | null;
}

const viNum = (v: any) => new Intl.NumberFormat('vi-VN').format(Number(v) || 0);

/** Nhãn dòng: qua labelFormatter nếu có, không thì lấy nguyên field (fallback '—'). */
function rowLabel(cfg: ReportSummaryConfig, item: any): string {
  const raw = item[cfg.labelField ?? 'name'];
  return cfg.labelFormatter ? cfg.labelFormatter(raw, item) : (raw || '—');
}

/** Ô tổng (text): để trống nếu cột không có tổng, giống footer bảng. */
function totalCell(cfg: ReportSummaryConfig, col: ReportSummaryCol): string {
  const v = cfg.totals?.[col.value];
  return v === undefined || v === null ? '' : viNum(v);
}

/** Copy bảng tổng hợp ra clipboard dạng text pipe-separated. */
export function copyReportSummary(cfg: ReportSummaryConfig, deps: ReportSummaryDeps): void {
  if (!passGuard(cfg, deps)) {
    return;
  }
  const {translate} = deps;
  const header = [
    translate.instant(cfg.firstColHeaderKey),
    ...cfg.cols.map(c => translate.instant(c.labelKey)),
  ].join(' | ');
  const lines: string[] = [
    `📊 ${translate.instant(cfg.titleKey)}`,
    '',
    header,
  ];
  cfg.items.forEach(item => {
    lines.push([rowLabel(cfg, item), ...cfg.cols.map(c => viNum(item[c.value]))].join(' | '));
  });
  lines.push('');
  lines.push([
    translate.instant('common.label.total'),
    ...cfg.cols.map(c => totalCell(cfg, c)),
  ].join(' | '));

  navigator.clipboard.writeText(lines.join('\n'))
    .then(() => deps.noti.successMsg(translate.instant('common.message.copy_success')))
    .catch(() => deps.noti.warnMsg(translate.instant('common.message.error')));
}

/** Export bảng tổng hợp ra Excel (client-side) từ dữ liệu đã tải. */
export function exportReportSummary(cfg: ReportSummaryConfig, deps: ReportSummaryDeps): void {
  if (!passGuard(cfg, deps)) {
    return;
  }
  const {translate} = deps;
  const title = translate.instant(cfg.titleKey);
  const headers = [
    translate.instant(cfg.firstColHeaderKey),
    ...cfg.cols.map(c => translate.instant(c.labelKey)),
  ];
  const dataRows: (string | number)[][] = cfg.items.map(item =>
    [rowLabel(cfg, item), ...cfg.cols.map(c => Number(item[c.value]) || 0)]);
  dataRows.push([
    translate.instant('common.label.total'),
    ...cfg.cols.map(c => {
      const v = cfg.totals?.[c.value];
      return v === undefined || v === null ? '' : Number(v) || 0;
    }),
  ]);

  const ws = XLSX.utils.aoa_to_sheet([[title], [], headers, ...dataRows]);
  ws['!cols'] = [{wch: 28}, ...cfg.cols.map(() => ({wch: 16}))];
  ws['!merges'] = [{s: {r: 0, c: 0}, e: {r: 0, c: headers.length - 1}}];
  const boldStyle = {font: {bold: true}};
  headers.forEach((_, i) => {
    const addr = XLSX.utils.encode_cell({r: 2, c: i});
    if (ws[addr]) {
      ws[addr].s = boldStyle;
    }
  });

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, safeSheetName(title));
  XLSX.writeFile(wb, `${cfg.filenameBase}-${rangeSlug(cfg.dateRange)}.xlsx`);
  deps.noti.successMsg(translate.instant('common.message.export_success'));
}

function passGuard(cfg: ReportSummaryConfig, deps: ReportSummaryDeps): boolean {
  if (!deps.permissionsService.getPermission(cfg.permission)) {
    deps.noti.warnMsg(deps.translate.instant('common.message.no_permission'));
    return false;
  }
  if (!cfg.items?.length) {
    deps.noti.warnMsg(deps.translate.instant('common.message.no_data'));
    return false;
  }
  return true;
}

/** Tên sheet Excel hợp lệ: bỏ ký tự cấm, tối đa 31 ký tự. */
export function safeSheetName(title: string): string {
  return title.replace(/[:\\/?*\[\]]/g, ' ').slice(0, 31) || 'Sheet1';
}

/** Slug khoảng ngày cho tên file, vd '01-07-2026_06-07-2026'. */
export function rangeSlug(range?: (Date | null | undefined)[] | null): string {
  const start = range?.[0];
  if (!start) {
    return 'export';
  }
  const end = range[1] ?? start;
  const fmt = (d: Date) => d.toLocaleDateString('vi-VN').replace(/\//g, '-');
  return `${fmt(start)}_${fmt(end)}`;
}
