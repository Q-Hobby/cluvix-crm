import { DestroyRef, inject, PLATFORM_ID, Service } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { NavigationEnd, Router } from '@angular/router';
import { filter } from 'rxjs/operators';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

// story-05 (epic teamchat-presence): "có tương tác thật hay không" trong khoảng heartbeat teamchat
// gần nhất (20s, `TC_PRESENCE_HEARTBEAT_MS` ở `teamchat.service.ts`). Ngưỡng *vắng mặt* phía BE là
// 10 PHÚT (`TcActiveWindowSec = 600`, `backend/pkg/define/teamchat.go`) ⇒ không cần độ phân giải cao.
// Đây là lý do KHÔNG lưu timestamp/lịch sử — chỉ MỘT cờ boolean, bị lật bởi tương tác thật và bị
// HEARTBEAT đọc-và-xoá (`consumeInteracted`), không phải bộ đếm thời gian tự trôi.
//
// Rủi ro R-F (architecture.md): service này gắn listener TOÀN CỤC (chạy trên mọi trang, dock teamchat
// mount ở topbar) ⇒ chi phí phải gần bằng 0:
//  ⛔ KHÔNG `setInterval`/`setTimeout` thứ hai — nhịp duy nhất đọc cờ là heartbeat teamchat đã có.
//  ⛔ KHÔNG dùng `mousemove` để theo dõi hoạt động toàn cục (repo chỉ dùng `mousemove` đúng MỘT chỗ:
//     drag slider so sánh ảnh — đừng biến nó thành pattern chung).
//  Mọi listener đăng ký `passive: true` — không được phép chặn/làm chậm sự kiện gốc của trình duyệt.
@Service()
export class UserActivityService {
  private destroyRef = inject(DestroyRef);
  private isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  /** ⛔ ĐÚNG MỘT cờ boolean — KHÔNG timestamp, KHÔNG mảng lịch sử (AC2). */
  private hasInteracted = false;

  constructor() {
    if (!this.isBrowser) return; // SSR/test không có `document` — an toàn bỏ qua, không gắn gì.

    const markInteracted = (): void => {
      this.hasInteracted = true;
    };
    const onVisibilityChange = (): void => {
      // Chỉ tính là tương tác khi tab QUAY LẠI (visible) — quay đi (hidden) không phải hành động của
      // người dùng đang dùng app, không lật cờ.
      if (document.visibilityState === 'visible') markInteracted();
    };

    document.addEventListener('pointerdown', markInteracted, { passive: true });
    document.addEventListener('keydown', markInteracted, { passive: true });
    document.addEventListener('visibilitychange', onVisibilityChange, { passive: true });

    this.destroyRef.onDestroy(() => {
      document.removeEventListener('pointerdown', markInteracted);
      document.removeEventListener('keydown', markInteracted);
      document.removeEventListener('visibilitychange', onVisibilityChange);
    });

    // Điều hướng route (bấm menu/link chuyển trang) cũng là tương tác thật.
    //
    // ⚠ `router.events?.` (KHÔNG bỏ optional chaining): rất nhiều spec trong repo mock `Router` chỉ
    // với `{ navigate: vi.fn() }` (không có `events`) — vì `UserActivityService` là `providedIn:
    // 'root'` nên bất kỳ service nào (kể cả không liên quan routing) inject nó xuyên qua
    // `TeamchatService` cũng làm constructor này chạy trong TestBed của họ. Thiếu optional chaining
    // ⇒ `Cannot read properties of undefined (reading 'pipe')` ngay lúc dựng service, sập HÀNG LOẠT
    // spec không hề đụng tới `UserActivityService`/routing (đã đo: 6 spec omnichat container đỏ oan
    // khi bỏ dòng này).
    inject(Router)
      .events?.pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(markInteracted);
  }

  /**
   * Gọi ĐÚNG 1 lần mỗi nhịp heartbeat teamchat (~20s) — đọc cờ rồi XOÁ ngay lập tức (AC6). KHÔNG gọi
   * lặp lại trong cùng 1 nhịp: lần gọi thứ hai luôn trả `false` dù người dùng vừa tương tác trước đó
   * trong nhịp này, vì cờ đã bị xoá ở lần gọi đầu.
   */
  consumeInteracted(): boolean {
    const v = this.hasInteracted;
    this.hasInteracted = false;
    return v;
  }
}
