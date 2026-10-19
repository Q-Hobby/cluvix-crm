import { computed, inject, Service} from '@angular/core';
import { authBusinessProfile } from '@/core/services/auth/auth.state';
import { DEFAULT_TIMEZONE } from '@/define';
import { LocaleFormatService } from '@/share/services/locale-format.service';

/**
 * =====================================================================
 * TimezoneService — Quản lý thời gian theo timezone của company
 * =====================================================================
 *
 * Tự động lấy timezone từ authBusinessProfile()?.timezone,
 * fallback DEFAULT_TIMEZONE (Asia/Ho_Chi_Minh).
 *
 * Quy ước:
 *   - Method kết thúc bằng "Unix"  → trả về unix timestamp (giây)
 *   - Method không có "Unix"       → trả về Date object
 *   - "Start"                      → 00:00:00.000 (đầu ngày)
 *   - "End"                        → 23:59:59.999 (cuối ngày)
 *
 * Sử dụng:
 *   // Inject sẵn trong BaseApp & BaseApiService
 *   // → Tất cả component/service kế thừa đều có this.timezoneService
 *
 *   // Gán default cho p-datePicker
 *   this.rfFilter.patchValue({
 *     dateRange: [this.timezoneService.monthStart(), this.timezoneService.now()]
 *   });
 *
 *   // Gửi lên API dạng unix timestamp (giây)
 *   const from = this.timezoneService.monthStartUnix();
 *   const to   = this.timezoneService.todayEndUnix();
 *
 *   // Filter cho p-table
 *   this.dt.filter(this.timezoneService.buildFilterTimestamp(value), 'created_at', 'between');
 *
 *   // Format hiển thị
 *   this.timezoneService.format(model.created_at); // "06/04/2026, 15:30:00"
 * =====================================================================
 */
@Service()
export class TimezoneService {
  readonly tz = computed(() => authBusinessProfile()?.timezone || DEFAULT_TIMEZONE);
  private localeFormat = inject(LocaleFormatService);

  /** Thời điểm hiện tại theo timezone company */
  now(): Date {
    return this.toTz(new Date());
  }

  /** Đầu ngày hôm nay 00:00:00 — Date */
  todayStart(): Date {
    const d = this.now();
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Cuối ngày hôm nay 23:59:59 — Date */
  todayEnd(): Date {
    const d = this.now();
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Đầu ngày hôm nay — unix timestamp (giây) */
  todayStartUnix(): number {
    const d = this.now();
    return this.wallClockToUnix(d.getFullYear(), d.getMonth(), d.getDate(), 0, 0, 0);
  }

  /** Cuối ngày hôm nay — unix timestamp (giây) */
  todayEndUnix(): number {
    const d = this.now();
    return this.wallClockToUnix(d.getFullYear(), d.getMonth(), d.getDate(), 23, 59, 59);
  }

  /** Đầu tháng này — Date */
  monthStart(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), n.getMonth(), 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Cuối tháng này — Date */
  monthEnd(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), n.getMonth() + 1, 0);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Đầu tháng này — unix timestamp (giây) */
  monthStartUnix(): number {
    const n = this.now();
    return this.wallClockToUnix(n.getFullYear(), n.getMonth(), 1, 0, 0, 0);
  }

  /** Cuối tháng này — unix timestamp (giây) */
  monthEndUnix(): number {
    const n = this.now();
    const lastDay = new Date(n.getFullYear(), n.getMonth() + 1, 0).getDate();
    return this.wallClockToUnix(n.getFullYear(), n.getMonth(), lastDay, 23, 59, 59);
  }

  /** Đầu tháng trước — Date */
  lastMonthStart(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), n.getMonth() - 1, 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Cuối tháng trước — Date */
  lastMonthEnd(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), n.getMonth(), 0);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Đầu năm nay — Date */
  yearStart(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), 0, 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Cuối năm nay — Date */
  yearEnd(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear(), 11, 31);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Đầu năm trước — Date */
  lastYearStart(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear() - 1, 0, 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Cuối năm trước — Date */
  lastYearEnd(): Date {
    const n = this.now();
    const d = new Date(n.getFullYear() - 1, 11, 31);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Ngày mai đầu ngày — Date */
  tomorrowStart(): Date {
    const d = this.now();
    d.setDate(d.getDate() + 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Ngày mai cuối ngày — Date */
  tomorrowEnd(): Date {
    const d = this.now();
    d.setDate(d.getDate() + 1);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /** Hôm qua đầu ngày — Date */
  yesterdayStart(): Date {
    const d = this.now();
    d.setDate(d.getDate() - 1);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /** Hôm qua cuối ngày — Date */
  yesterdayEnd(): Date {
    const d = this.now();
    d.setDate(d.getDate() - 1);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  /**
   * Unix timestamp (giây) từ Date (wall-clock parts) + giờ/phút/giây theo company timezone.
   * Dùng khi biết ngày từ date object và giờ từ string (vd slot "15:00" trong schedule byhour).
   */
  dateHmsToUnix(date: Date, h: number, m = 0, s = 0): number {
    return this.wallClockToUnix(date.getFullYear(), date.getMonth(), date.getDate(), h, m, s);
  }

  /** Unix timestamp (giây) từ Date theo company timezone */
  toUnix(date: Date | string | null, mode: 'start' | 'end' = 'end'): number | undefined {
    if (!date) return undefined;
    const d = this.toTz(new Date(date));
    if (isNaN(d.getTime())) return undefined;
    const h = mode === 'start' ? 0 : 23;
    const m = mode === 'start' ? 0 : 59;
    const s = mode === 'start' ? 0 : 59;
    return this.wallClockToUnix(d.getFullYear(), d.getMonth(), d.getDate(), h, m, s);
  }

  /**
   * Unix timestamp (giây) → Date wall-clock theo company timezone — hàm ngược của toUnix().
   * Dùng khi khôi phục filter khoảng ngày từ URL về lại control p-datePicker.
   */
  fromUnix(unix: number | string | null | undefined): Date | null {
    if (unix === null || unix === undefined || unix === '') return null;
    const n = Number(unix);
    if (isNaN(n)) return null;
    return this.toTz(new Date(n < 1e12 ? n * 1000 : n));
  }

  /**
   * Số NGÀY LỊCH từ `date` tới hiện tại, tính theo **timezone company** (0 = hôm nay, 1 = hôm qua).
   * Âm khi mốc nằm ở tương lai (lệch đồng hồ server/máy trạm) — caller nên coi `<= 0` là hôm nay.
   *
   * VÌ SAO KHÔNG lấy `(now - date) / 86400000`: cách đó đo khoảng TRÔI QUA chứ không phải ranh giới
   * ngày. Hai hệ quả đã gặp thật (omnichat 2026-08-16): mốc do server sinh nhanh hơn trình duyệt vài
   * ms cho ra -1 ⇒ tin vừa tới hiện NGÀY thay vì GIỜ; và tin 23:50 hôm qua xem lúc 00:10 cho ra 0 ⇒
   * hiện giờ thay vì "hôm qua".
   *
   * VÌ SAO PHẢI THEO TZ COMPANY: ranh giới ngày phải trùng với timezone mà giờ đang được HIỂN THỊ
   * (format() dùng tz company). Lấy ngày theo trình duyệt trong khi in giờ theo tz company thì ở gần
   * nửa đêm sẽ ra "hôm qua 23:50" bên cạnh một mốc mà hệ thống coi là hôm nay.
   */
  calendarDaysAgo(date: Date | number | string | null, now: Date = new Date()): number {
    if (!date) return 0;
    const d = typeof date === 'number'
      ? new Date(date < 1e12 ? date * 1000 : date)
      : new Date(date);
    if (isNaN(d.getTime())) return 0;
    const startOfDay = (x: Date) => {
      const t = this.toTz(x);
      return new Date(t.getFullYear(), t.getMonth(), t.getDate()).getTime();
    };
    return Math.round((startOfDay(now) - startOfDay(d)) / 86400000);
  }

  /** Format Date theo company timezone */
  format(date: Date | number | string | null, options?: Intl.DateTimeFormatOptions): string {
    if (!date) return '';
    const d = typeof date === 'number'
      ? new Date(date < 1e12 ? date * 1000 : date)
      : new Date(date);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleString(this.localeFormat.currentLocale(), { timeZone: this.tz(), ...options });
  }

  /**
   * Build filter timestamp theo company timezone (dùng cho p-table filter).
   *
   * KHÔNG được đi qua `toUnix()` (nó gọi `toTz()` bên trong): mọi Date thực tế truyền vào đây
   * đều ĐÃ là wall-clock Date — output của `todayStart()`/`monthStart()`/`fromUnix()`, hoặc giá
   * trị người dùng chọn trên `p-datePicker` (y/m/d là ý định lịch, không phải một "moment" cần
   * quy chiếu tz). `toTz()` không idempotent — gọi thêm lần nữa lên Date loại này lệch đúng
   * `companyOffset − browserOffset`, có thể lệch nguyên 1 ngày khi 2 offset cách xa nhau.
   * Dùng `dateHmsToUnix()` (trích y/m/d trực tiếp) — cùng công thức `todayStartUnix()` đang dùng.
   */
  buildFilterTimestamp(value: any): [number, number] | null {
    if (!value) return null;
    const [start, end] = Array.isArray(value) ? value : [value];
    if (!start) return null;
    const startDate = new Date(start);
    const endDate = end ? new Date(end) : startDate;
    return [
      this.dateHmsToUnix(startDate, 0, 0, 0),
      this.dateHmsToUnix(endDate, 23, 59, 59),
    ];
  }

  /**
   * Convert Date sang timezone company.
   * Dùng Intl API để lấy các parts (year, month, day, hour...) theo timezone,
   * rồi tạo Date mới từ các parts đó.
   *
   * LƯU Ý: Date object trả về có getTime() tương ứng với wall-clock time
   * của company timezone, nhưng JS vẫn coi nó là browser timezone.
   * → Dùng cho setHours/getFullYear/getMonth/getDate (lấy đúng ngày/tháng/năm)
   * → KHÔNG dùng getTime() trực tiếp để gửi API.
   */
  private toTz(date: Date): Date {
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: this.tz(),
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
    const parts = formatter.formatToParts(date);
    const get = (type: string) => parseInt(parts.find((p) => p.type === type)?.value || '0', 10);
    return new Date(get('year'), get('month') - 1, get('day'), get('hour'), get('minute'), get('second'));
  }

  /**
   * Tính unix timestamp (giây) cho một wall-clock time trong company timezone.
   * VD: "2026-04-06 00:00:00 Asia/Tokyo" → đúng timestamp UTC.
   */
  private wallClockToUnix(year: number, month: number, day: number, h = 0, m = 0, s = 0): number {
    // Tạo ISO string giả với offset = 0, rồi dùng Intl để tìm offset thực
    const probe = new Date(Date.UTC(year, month, day, h, m, s));
    const tzStr = this.tz();

    // Lấy wall-clock time tại timezone company cho thời điểm probe
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: tzStr,
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    });
    const parts = formatter.formatToParts(probe);
    const g = (type: string) => parseInt(parts.find((p) => p.type === type)?.value || '0', 10);
    const wallAtProbe = Date.UTC(g('year'), g('month') - 1, g('day'), g('hour'), g('minute'), g('second'));

    // offset = wallAtProbe - probe (ms)
    const offsetMs = wallAtProbe - probe.getTime();

    // Target wall-clock → UTC = target - offset
    const targetWallMs = Date.UTC(year, month, day, h, m, s);
    return Math.floor((targetWallMs - offsetMs) / 1000);
  }
}
