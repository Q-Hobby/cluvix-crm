import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { BaseIndexedDBService } from './base-indexed-db.service';
import { TABLE_REGISTRY } from '@/indexeddb/config/table-registry';

/**
 * story-05 (epic notification-debt-cleanup) AC3 — nợ pre-existing lộ ra ở story-11 (deferred-work.md
 * dòng ~68): `initDB()` gán `initPromise` rồi KHÔNG reset trong nhánh `onerror`/`onblocked` ⇒ một lần
 * open fail (điển hình `VersionError` sau khi bump `INDEXEDDB_DB_VERSION` ở tab cũ) khoá VĨNH VIỄN
 * mọi thao tác IndexedDB của tab đó tới khi F5, vì `initDB()` gọi sau chỉ trả lại đúng promise ĐÃ
 * reject này (dòng `if (this.initPromise) return this.initPromise;`) mà không thử `indexedDB.open`
 * lại. Test khoá qua ĐẾM số lần `indexedDB.open` được gọi — không mock nội bộ transaction, vì điểm
 * chốt của bug nằm đúng ở việc CÓ mở lại kết nối hay không.
 */
class FakeOpenRequest {
  onerror: (() => void) | null = null;
  onblocked: (() => void) | null = null;
  onsuccess: (() => void) | null = null;
  onupgradeneeded: ((event: unknown) => void) | null = null;
  error = new Error('boom');
  result: unknown = { objectStoreNames: { contains: () => true }, close: () => {}, onclose: null, onversionchange: null };
}

describe('BaseIndexedDBService.initDB — reset initPromise khi open fail (story-05 AC3)', () => {
  let requests: FakeOpenRequest[];
  let openSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    requests = [];
    openSpy = vi.fn(() => {
      const req = new FakeOpenRequest();
      requests.push(req);
      return req as unknown as IDBOpenDBRequest;
    });
    vi.stubGlobal('indexedDB', { open: openSpy });

    TestBed.configureTestingModule({
      providers: [{ provide: TABLE_REGISTRY, useValue: [] }],
    });
  });

  afterEach(() => {
    TestBed.resetTestingModule();
    vi.unstubAllGlobals();
  });

  it('open lần 1 fail (onerror) rồi initDB() lần sau THỬ MỞ LẠI, không kẹt vĩnh viễn', async () => {
    const svc = TestBed.inject(BaseIndexedDBService);

    const first = svc.initDB();
    // Kích onerror ngay khi request vừa được tạo (giả lập VersionError đồng bộ).
    requests[0].onerror?.();
    await expect(first).rejects.toThrow('boom');
    expect(openSpy).toHaveBeenCalledTimes(1);

    // Lần gọi sau: nếu `initPromise` KHÔNG được reset, service trả lại CHÍNH promise đã reject ở
    // trên mà KHÔNG gọi `indexedDB.open` lần nữa ⇒ openSpy vẫn dừng ở 1 lần (bug tái hiện được ở
    // đây nếu ai lỡ gỡ dòng reset).
    const second = svc.initDB();
    expect(openSpy).toHaveBeenCalledTimes(2);
    requests[1].onsuccess?.();
    await expect(second).resolves.toBeUndefined();
  });

  it('open lần 1 fail (onblocked) rồi initDB() lần sau cũng THỬ MỞ LẠI', async () => {
    const svc = TestBed.inject(BaseIndexedDBService);

    const first = svc.initDB();
    requests[0].onblocked?.();
    await expect(first).rejects.toThrow('IDB open blocked by another tab');
    expect(openSpy).toHaveBeenCalledTimes(1);

    const second = svc.initDB();
    expect(openSpy).toHaveBeenCalledTimes(2);
    requests[1].onsuccess?.();
    await expect(second).resolves.toBeUndefined();
  });
});
