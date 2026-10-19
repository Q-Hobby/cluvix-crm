import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { QrCodeComponent } from 'ng-qrcode';

/**
 * Khối chữ ký cho mẫu in bệnh án.
 *
 * - Khi BAĐT đã ký số (BE stamp `signed_at`/`signed_by`/`signature_hash`):
 *   hiển thị badge "Đã ký số", thời gian ký, tên người ký, hash rút gọn, QR code
 *   dẫn về trang verify (theo TT 46/2018-BYT + NĐ 130/2018-NĐ-CP).
 *
 * - Khi chưa ký (manual sign / draft): giữ pattern cũ
 *   "Ký, ghi rõ họ tên" + tên BS.
 *
 * LƯU Ý: dùng **signal input** (`input()`) thay cho `@Input` cổ điển — vì bản in
 * load record BẤT ĐỒNG BỘ; nếu `model` là @Input thường thì `computed()` đọc nó
 * sẽ KHÔNG tính lại khi model về (computed chỉ track signal) → cache `null` →
 * không bao giờ hiện chữ ký dù đã ký. Signal input cho computed track đúng.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-signature-block',
  standalone: true,
  imports: [CommonModule, QrCodeComponent],
  template: `
    <div class="text-center">
      @if (signedAt(); as ts) {
        <!-- Signed state -->
        <div class="inline-flex items-center gap-1 px-2 py-0.5 border border-emerald-600 rounded text-emerald-700 text-[10px] font-semibold">
          <span>✓ ĐÃ KÝ SỐ</span>
        </div>
        <div class="text-[11px] mt-1">{{ ts | date:'HH:mm dd/MM/yyyy' }}</div>
        <div class="font-semibold mt-0.5">{{ signerName() }}</div>
        @if (showVerifyQr && verifyUrl()) {
          <div class="flex justify-center mt-1">
            <qr-code [value]="verifyUrl()" size="64" errorCorrectionLevel="M"/>
          </div>
          <div class="text-[8px] text-gray-500 mt-0.5 font-mono">{{ hashShort() }}</div>
          <div class="text-[8px] text-gray-500">Quét QR để xác thực</div>
        }
      } @else {
        <!-- Unsigned / paper sign — dòng ngày + chức danh do template cha in (tránh lặp) -->
        <i>(Ký, ghi rõ họ tên)</i>
        <div class="mt-12">{{ doctorName() }}</div>
      }
    </div>
  `,
})
export class PrintSignatureBlock {
  /** Bệnh án — cần có signed_at, signed_by, signature_hash từ BE. */
  readonly model = input<any>(null);
  /** Tên BS điều trị — hiển thị ở chế độ ký giấy (fallback cho người ký). */
  readonly doctorName = input<string | null>(null);
  /** Verify route trên FE — BE phải implement endpoint tương ứng. */
  readonly verifyRoute = input<string>('/emr/verify-signature');

  /**
   * QR xác thực — bật khi trang + endpoint verify (BE-054) đã sẵn sàng.
   * Trang public `/emr/verify-signature/:id` + `GET /emr/verify-signature/:id?hash=`
   * đã có (handoff plan/18) nên QR trỏ tới trang xác thực thật.
   */
  protected readonly showVerifyQr = true;

  signedAt = computed<Date | null>(() => {
    const ts = this.model()?.signed_at;
    if (!ts) return null;
    return new Date(ts * 1000);
  });

  /**
   * Tên người KÝ thực tế. Ưu tiên `signedByInfo` (resolve từ `signed_by` — người
   * thật sự đặt bút ký, có thể KHÁC bác sĩ điều trị trong mô hình ký per-doctor
   * BE-040), fallback về `doctorName` (BS điều trị) rồi '—'.
   */
  signerName = computed<string>(() => this.model()?.signedByInfo?.fullname || this.doctorName() || '—');

  hashShort = computed(() => {
    const h = this.model()?.signature_hash as string | undefined;
    if (!h) return '';
    return h.length > 16 ? h.slice(0, 16) + '…' : h;
  });

  verifyUrl = computed(() => {
    const id = this.model()?.id;
    const hash = this.model()?.signature_hash;
    if (!id || !hash) return '';
    const origin = typeof window !== 'undefined' ? window.location.origin : '';
    return `${origin}${this.verifyRoute()}/${id}?hash=${encodeURIComponent(hash)}`;
  });
}
