import { TestBed } from '@angular/core/testing';
import { NavigationEnd, Router } from '@angular/router';
import { Subject } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { UserActivityService } from './user-activity.service';

/**
 * Story-05 (epic teamchat-presence) — cờ boolean "có tương tác thật hay không" của heartbeat teamchat.
 *
 * ⚠ Không có UI/component nào đọc `hasInteracted` (nó là state nội bộ tiêu thụ bởi
 * `TeamchatService._sendHeartbeat`) nên spec này gọi thẳng `consumeInteracted()` — đường dùng THẬT
 * của sản phẩm — thay vì đọc field private qua `as unknown as {...}` (dễ trôi khỏi hành vi thật khi
 * refactor tên field nội bộ).
 */
describe('UserActivityService — cờ tương tác cho heartbeat teamchat (story-05)', () => {
  let svc: UserActivityService;
  let routerEvents: Subject<unknown>;

  beforeEach(() => {
    routerEvents = new Subject();
    TestBed.configureTestingModule({
      providers: [UserActivityService, { provide: Router, useValue: { events: routerEvents } }],
    });
    svc = TestBed.inject(UserActivityService);
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('AC2: mặc định chưa có tương tác nào — consumeInteracted() trả false', () => {
    expect(svc.consumeInteracted()).toBe(false);
  });

  it('AC3: pointerdown lật cờ thành true', () => {
    document.dispatchEvent(new Event('pointerdown'));
    expect(svc.consumeInteracted()).toBe(true);
  });

  it('AC3: keydown lật cờ thành true', () => {
    document.dispatchEvent(new Event('keydown'));
    expect(svc.consumeInteracted()).toBe(true);
  });

  it('AC3: visibilitychange CHỈ lật cờ khi tab đang ở trạng thái visible (quay lại), không phải khi ẩn', () => {
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
    document.dispatchEvent(new Event('visibilitychange'));
    expect(svc.consumeInteracted()).toBe(false);

    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
    document.dispatchEvent(new Event('visibilitychange'));
    expect(svc.consumeInteracted()).toBe(true);
  });

  it('AC3: điều hướng route (NavigationEnd) lật cờ thành true', () => {
    routerEvents.next(new NavigationEnd(1, '/a', '/a'));
    expect(svc.consumeInteracted()).toBe(true);
  });

  it('AC6: consumeInteracted() đọc-và-XOÁ — gọi lần 2 liên tiếp trả false dù không có tương tác mới', () => {
    document.dispatchEvent(new Event('pointerdown'));
    expect(svc.consumeInteracted()).toBe(true);
    expect(svc.consumeInteracted()).toBe(false);
  });

  it('AC5: mousemove KHÔNG được coi là tương tác (chỉ dùng riêng cho drag slider so sánh ảnh)', () => {
    document.dispatchEvent(new Event('mousemove'));
    expect(svc.consumeInteracted()).toBe(false);
  });

  it('AC7: huỷ service (destroy) gỡ hết listener — tương tác sau đó không còn lật được cờ', () => {
    TestBed.resetTestingModule();
    expect(svc.consumeInteracted()).toBe(false);
    document.dispatchEvent(new Event('pointerdown'));
    // Listener đã bị gỡ ở onDestroy → dispatch sau khi service bị huỷ không còn tác dụng.
    expect(svc.consumeInteracted()).toBe(false);
  });
});
