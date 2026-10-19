import {Identifiable} from '@/core/interface/identifiable';
import {BaseListV2} from '@/share/base/base-list-v2';
import {
  copyReportSummary,
  exportReportSummary,
  ReportSummaryConfig,
  ReportSummaryDeps,
} from '@/share/utils/report-summary-export.util';

/**
 * Base cho các báo cáo doanh thu tổng hợp (report-revenue/by-*) có cùng cấu trúc:
 * bảng gồm cột nhãn + các cột số + dòng Tổng, dữ liệu tải hết ở FE.
 *
 * Component con chỉ cần cấu hình `reportSummaryCfg()`; copy/export dùng chung ở đây.
 * Xem helper: `@/share/utils/report-summary-export.util`.
 */
export abstract class BaseRevenueSummaryReport<T extends Identifiable> extends BaseListV2<T> {
  /** Cấu hình copy/export cho báo cáo cụ thể. */
  protected abstract reportSummaryCfg(): ReportSummaryConfig;

  private reportSummaryDeps(): ReportSummaryDeps {
    return {
      translate: this.translate,
      noti: this.noti,
      permissionsService: this.permissionsService,
    };
  }

  copyData(): void {
    copyReportSummary(this.reportSummaryCfg(), this.reportSummaryDeps());
  }

  exportSummary(): void {
    exportReportSummary(this.reportSummaryCfg(), this.reportSummaryDeps());
  }
}
