import { ChangeDetectionStrategy, Component, Input } from '@angular/core';

/**
 * Watermark cảnh báo cho bản in bệnh án CHƯA KÝ SỐ hợp lệ (thiếu signed_at/
 * signature_hash/sign_file_url) — kể cả record_status='completed' nhưng ký tay.
 * Hiển thị chéo toàn trang khi in để tránh nhầm là BAĐT có giá trị pháp lý
 * (TT 46/2018, NĐ 130/2018).
 *
 * Usage:
 *   <app-draft-watermark [show]="isDraft()"/>   // isDraft = chưa ký số
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-draft-watermark',
  standalone: true,
  template: `
    @if (show) {
      <div class="draft-watermark" aria-hidden="true">{{ text }}</div>
    }
  `,
  styles: [`
    .draft-watermark {
      position: fixed;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%) rotate(-30deg);
      font-size: 7.3846rem;
      font-weight: 900;
      color: rgba(220, 38, 38, 0.18);
      letter-spacing: 8px;
      pointer-events: none;
      white-space: nowrap;
      user-select: none;
      z-index: 9999;
    }
    @media print {
      .draft-watermark {
        position: fixed;
        color: rgba(220, 38, 38, 0.25) !important;
        -webkit-print-color-adjust: exact;
        print-color-adjust: exact;
      }
    }
  `],
})
export class DraftWatermark {
  @Input() show: boolean = false;
  @Input() text: string = 'BẢN CHƯA KÝ SỐ';
}
