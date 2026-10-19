import { computed, Service, signal } from '@angular/core';

/**
 * Sự kiện Chrome/Edge bắn khi trang đủ điều kiện cài PWA (manifest + service worker + HTTPS).
 * Chưa có trong lib.dom của TypeScript nên tự khai.
 */
interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>;
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

/**
 * Cài Cluvix thành ứng dụng desktop (PWA) — nút "Cài ứng dụng" trên AppTopbar.
 *
 * - Chrome/Edge: giữ lại `beforeinstallprompt` rồi gọi `prompt()` khi user bấm nút.
 *   Sự kiện bắn RẤT SỚM — trước cả khi Angular bootstrap xong ⇒ bắt bằng script inline trong
 *   index.html (window.__cluvixInstallPrompt), service chỉ đọc lại. Service inject ở AppComponent.
 * - Safari (macOS 14+): không có API — user tự vào File → Add to Dock ⇒ topbar hiện hướng dẫn.
 * - Firefox desktop: không hỗ trợ cài PWA ⇒ không hiện nút.
 * - Đang chạy trong app đã cài (display-mode standalone) ⇒ không hiện nút.
 */
@Service()
export class PwaInstallService {
  private deferredPrompt: BeforeInstallPromptEvent | null = null;

  /**
   * Trình duyệt đã báo "cài được" (có ít nhất 1 lần beforeinstallprompt). GIỮ true tới khi cài
   * xong — KHÔNG hạ khi user bấm Hủy, nếu không nút biến mất dù app chưa được cài.
   */
  private readonly installable = signal(false);
  private readonly isStandalone = signal(this.detectStandalone());
  /** Safari trên Mac — cài bằng tay qua menu File → Add to Dock. */
  readonly isSafariMac = this.detectSafariMac();

  /** Hiện nút cài khi chưa chạy trong app đã cài VÀ trình duyệt có đường cài. */
  readonly showInstall = computed(() => !this.isStandalone() && (this.installable() || this.isSafariMac));

  constructor() {
    if (typeof window === 'undefined') return;

    // Event được bắt bằng script inline trong index.html (bắn trước khi Angular bootstrap xong,
    // listener gắn ở đây sẽ lỡ). Đọc cái đã bắt được + nghe các lần bắn sau (sau khi user từ chối).
    const w = window as Window & { __cluvixInstallPrompt?: BeforeInstallPromptEvent };
    const takeCaptured = () => {
      if (!w.__cluvixInstallPrompt) return;
      this.deferredPrompt = w.__cluvixInstallPrompt;
      this.installable.set(true);
    };
    takeCaptured();
    window.addEventListener('cluvix-installprompt', takeCaptured);

    window.addEventListener('appinstalled', () => {
      w.__cluvixInstallPrompt = undefined;
      this.deferredPrompt = null;
      this.installable.set(false);
      this.isStandalone.set(true);
    });

    window
      .matchMedia('(display-mode: standalone)')
      .addEventListener('change', (e) => this.isStandalone.set(e.matches || this.detectStandalone()));
  }

  /**
   * Mở hộp thoại cài của trình duyệt (Chrome/Edge).
   * Trả `unavailable` khi không có prompt — Safari, hoặc Chrome sau khi user đã Hủy 1 lần (event
   * dùng 1 lần, Chrome chỉ bắn lại ở lần tải trang sau) — caller tự hiện hướng dẫn cài tay.
   */
  async install(): Promise<'accepted' | 'dismissed' | 'unavailable'> {
    const prompt = this.deferredPrompt;
    if (!prompt) return 'unavailable';

    // prompt() chỉ gọi được 1 lần/event ⇒ bỏ event đi, nhưng KHÔNG ẩn nút (xem `installable`).
    (window as Window & { __cluvixInstallPrompt?: BeforeInstallPromptEvent }).__cluvixInstallPrompt = undefined;
    this.deferredPrompt = null;
    await prompt.prompt();
    const { outcome } = await prompt.userChoice;
    return outcome;
  }

  private detectStandalone(): boolean {
    if (typeof window === 'undefined') return false;
    return (
      window.matchMedia('(display-mode: standalone)').matches ||
      (navigator as Navigator & { standalone?: boolean }).standalone === true
    );
  }

  private detectSafariMac(): boolean {
    if (typeof navigator === 'undefined') return false;
    const ua = navigator.userAgent;
    const isMac = /Macintosh/.test(ua) && navigator.maxTouchPoints === 0;
    const isSafari = /Safari\//.test(ua) && !/Chrome|Chromium|CriOS|Edg|OPR|Firefox|FxiOS/.test(ua);
    return isMac && isSafari;
  }
}
