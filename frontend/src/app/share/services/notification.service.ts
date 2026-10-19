// src/app/shared/services/notification.service.ts
import { inject, Service} from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { MessageService } from 'primeng/api';

/** Thời gian sống của 1 toast — cũng là CỬA SỔ GỘP trùng bên dưới. */
const TOAST_LIFE_MS = 3000;

@Service()
export class NotificationService {
  private messageService = inject(MessageService);
  private translate = inject(TranslateService);

  // Chống spam toast trùng (2026-08-16): một màn gọi N API song song, cả N cùng chết vì MỘT nguyên
  // nhân (mất mạng / server không phản hồi / 500) → interceptor bắn N toast chữ y hệt nhau, phủ kín
  // màn hình mà không thêm thông tin gì. Gộp ở ĐÂY chứ không ở interceptor vì đây là cửa duy nhất
  // mọi nơi trong app đi qua để hiện toast — vá ở interceptor chỉ chữa được đúng lỗi HTTP.
  //
  // Quy tắc: chỉ chặn khi TRÙNG TUYỆT ĐỐI (cùng severity + summary + detail) và bản cũ vẫn đang
  // hiện. Hai lỗi khác nội dung (vd 2 message validate khác nhau) KHÔNG bị gộp — vẫn hiện đủ.
  //
  // Cửa sổ TRƯỢT: mỗi lần chặn lại gia hạn mốc, nên chuỗi API lỗi rải rác vài giây chỉ tốn 1 toast
  // cho cả đợt; im lặng đủ 3s rồi lỗi lại → toast mới (người dùng vẫn biết là lỗi lặp lại).
  private lastShownAt = new Map<string, number>();

  private get title() {
    return this.translate.instant('common.label.notification');
  }

  private show(severity: 'success' | 'error' | 'info' | 'warn', summary: string, detail?: string) {
    const now = Date.now();
    const key = `${severity}|${summary}|${detail ?? ''}`;
    const prev = this.lastShownAt.get(key);
    this.lastShownAt.set(key, now);
    if (prev != null && now - prev < TOAST_LIFE_MS) return;

    // Dọn mốc đã hết hạn: map chỉ giữ vài key đang trong cửa sổ, không phình theo phiên làm việc.
    for (const [k, at] of this.lastShownAt) {
      if (now - at >= TOAST_LIFE_MS && k !== key) this.lastShownAt.delete(k);
    }

    this.messageService.add({ severity, summary, detail, life: TOAST_LIFE_MS });
  }

  success(summary: string, detail?: string) {
    this.show('success', summary, detail);
  }

  // Dùng khi có response từ BE — tự lấy message, tự dùng i18n title
  successRes(res: any) {
    this.success(this.title, res?.message ?? res?.data?.message);
  }

  // Dùng khi có message sẵn — không cần inject translate ở component
  successMsg(detail: string) {
    this.success(this.title, detail);
  }

  error(summary: string, detail?: string) {
    this.show('error', summary, detail);
  }

  errorMsg(detail: string) {
    this.error(this.translate.instant('common.status.error'), detail);
  }

  info(summary: string, detail?: string) {
    this.show('info', summary, detail);
  }

  infoMsg(detail: string) {
    this.info(this.title, detail);
  }

  warn(summary: string, detail?: string) {
    this.show('warn', summary, detail);
  }

  warnMsg(detail: string) {
    this.warn(this.translate.instant('common.label.warning'), detail);
  }

  clear() {
    // Xoá luôn mốc gộp: sau khi dọn sạch màn hình thì toast trùng phải được hiện lại ngay, không
    // thì có lúc gọi clear() xong bắn lại đúng message đó mà màn hình im lặng.
    this.lastShownAt.clear();
    this.messageService.clear();
  }
}
