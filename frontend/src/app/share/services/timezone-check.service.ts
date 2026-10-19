import { inject, Service} from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { TimezoneService } from '@/core/services/timezone.service';

@Service()
export class TimezoneCheckService {
  public dialogVisible$ = new BehaviorSubject<boolean>(false);
  private timezoneService = inject(TimezoneService);
  private hasChecked = false;

  /**
   * Kiểm tra browser timezone có khớp với company timezone không.
   * Nếu UTC offset lệch → hiện cảnh báo.
   *
   * Gọi 1 lần sau khi login / load app.
   *
   * LƯU Ý: KHÔNG dùng timezoneService.now().getTime() để so sánh vì toTz() trả
   * Date với getTime() = Date.now() + (companyOffset − browserOffset), nên skew
   * = offset difference chứ không phải clock accuracy — false positive khi company
   * timezone trong DB ≠ Asia/Ho_Chi_Minh dù browser đúng UTC+7.
   */
  check(): boolean {
    if (this.hasChecked) return true;
    this.hasChecked = true;

    const now = new Date();
    const companyOffset = this.tzOffsetMinutes(this.timezoneService.tz(), now);
    const browserOffset = -now.getTimezoneOffset(); // positive = ahead of UTC

    if (companyOffset !== browserOffset) {
      this.dialogVisible$.next(true);
      return false;
    }
    return true;
  }

  /**
   * Tính UTC offset (phút) của một timezone tại thời điểm date.
   * VD: "Asia/Ho_Chi_Minh" → 420, "UTC" → 0.
   */
  private tzOffsetMinutes(tz: string, date: Date): number {
    const f = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    });
    const p = f.formatToParts(date);
    const g = (type: string) => parseInt(p.find(x => x.type === type)?.value ?? '0', 10);
    const wallMs = Date.UTC(g('year'), g('month') - 1, g('day'), g('hour'), g('minute'), g('second'));
    return Math.round((wallMs - date.getTime()) / 60000);
  }

  close() {
    this.dialogVisible$.next(false);
  }

  /** Reset để check lại (khi user đổi company) */
  reset() {
    this.hasChecked = false;
  }
}
