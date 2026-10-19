import * as XLSX from 'xlsx-js-style';
import {TranslateService} from '@ngx-translate/core';

import {NotificationService} from '@/share/services/notification.service';
import {PermissionsService} from '@/share/services/permissions.service';
import {rangeSlug, safeSheetName} from '@/share/utils/report-summary-export.util';

/**
 * Helper copy/export cho báo cáo dạng DANH SÁCH chi tiết (list-*), khác summary:
 * mỗi cột render theo logic template riêng (nhãn, ngày, tiền...) → component cung cấp
 * hàm `cell(item, colValue)` trả về chuỗi hiển thị. Không có tổng theo từng cột;
 * chỉ 1 tổng tuỳ chọn cho 1 cột (vd tổng tiền cọc/giảm trừ).
 *
 * Bảng phân trang server-side → component tự fetch-all trước rồi truyền `items` đầy đủ.
 */

export interface ReportListCol {
  value: string;
  /** i18n key cho tiêu đề cột. Nếu không có, dùng `label` hoặc `value`. */
  labelKey?: string;
  label?: string;
}

export interface ReportListDeps {
  translate: TranslateService;
  noti: NotificationService;
  permissionsService: PermissionsService;
}

export interface ReportListConfig {
  items: any[];
  /** Các cột đang hiển thị theo thứ tự. */
  cols: ReportListCol[];
  /** Render 1 ô theo đúng logic template của component. */
  cell: (item: any, colValue: string) => string;
  titleKey: string;
  filenameBase: string;
  permission: string;
  dateRange?: (Date | null | undefined)[] | null;
  /** Tổng cho 1 cột (đã format sẵn), vd tổng tiền. */
  total?: {colValue: string; text: string};
}

function headerLabels(cfg: ReportListConfig, translate: TranslateService): string[] {
  return cfg.cols.map(c => (c.labelKey ? translate.instant(c.labelKey) : (c.label ?? c.value)));
}

function totalRow(cfg: ReportListConfig, translate: TranslateService): string[] | null {
  if (!cfg.total) {
    return null;
  }
  return cfg.cols.map((c, i) => {
    if (i === 0) {
      return translate.instant('common.label.total');
    }
    return c.value === cfg.total!.colValue ? cfg.total!.text : '';
  });
}

function passGuard(cfg: ReportListConfig, deps: ReportListDeps): boolean {
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

/** Copy danh sách ra clipboard dạng text pipe-separated. */
export function copyReportList(cfg: ReportListConfig, deps: ReportListDeps): void {
  if (!passGuard(cfg, deps)) {
    return;
  }
  const {translate} = deps;
  const lines: string[] = [
    `📋 ${translate.instant(cfg.titleKey)}`,
    '',
    headerLabels(cfg, translate).join(' | '),
  ];
  cfg.items.forEach(item => {
    lines.push(cfg.cols.map(c => cfg.cell(item, c.value)).join(' | '));
  });
  const total = totalRow(cfg, translate);
  if (total) {
    lines.push('');
    lines.push(total.join(' | '));
  }

  navigator.clipboard.writeText(lines.join('\n'))
    .then(() => deps.noti.successMsg(translate.instant('common.message.copy_success')))
    .catch(() => deps.noti.warnMsg(translate.instant('common.message.error')));
}

/** Export danh sách ra Excel (client-side). */
export function exportReportList(cfg: ReportListConfig, deps: ReportListDeps): void {
  if (!passGuard(cfg, deps)) {
    return;
  }
  const {translate} = deps;
  const title = translate.instant(cfg.titleKey);
  const headers = headerLabels(cfg, translate);
  const dataRows: string[][] = cfg.items.map(item => cfg.cols.map(c => cfg.cell(item, c.value)));
  const total = totalRow(cfg, translate);
  if (total) {
    dataRows.push(total);
  }

  const ws = XLSX.utils.aoa_to_sheet([[title], [], headers, ...dataRows]);
  ws['!cols'] = cfg.cols.map(() => ({wch: 20}));
  ws['!merges'] = [{s: {r: 0, c: 0}, e: {r: 0, c: Math.max(headers.length - 1, 0)}}];
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
