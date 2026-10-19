import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MasterDataRegistryService } from './master-data-registry.service';
import { TableServiceFactory } from './table-service-factory';
import { TABLE_REGISTRY, TableConfig } from '@/indexeddb/config/table-registry';
import { CacheInvalidationRegistry } from '@/indexeddb/config/service/cache-invalidation-registry.service';

/**
 * story-11 (epic notification-core-v2) — F2 (QA Pass 1, patch low): guard `localOnly` ban đầu chỉ
 * đứng ở `syncSingleTable`. `invalidateAndSync` gọi `service.save([])` (= xoá trắng store) TRƯỚC khi
 * gọi `syncSingleTable`, nên với 1 store per-user (`notifications`) cửa `invalidateAndSync` vẫn xoá
 * dữ liệu dù `syncSingleTable` trả sớm ngay sau đó. Test này khoá CẢ HAI cửa bằng service.save spy
 * thật (không mock `invalidateAndSync` chính nó) — mutation bỏ guard ở `invalidateAndSync` phải làm
 * test (a) đỏ.
 */
describe('MasterDataRegistryService — guard `localOnly` chặn store per-user (story-11 F2)', () => {
  const LOCAL_ONLY: TableConfig = { name: 'notifications', keyPath: 'id', localOnly: true };
  const NORMAL: TableConfig = { name: 'units', keyPath: 'id' };

  let httpMock: HttpTestingController;
  let saveSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    saveSpy = vi.fn().mockResolvedValue(undefined);
    const fakeService = { save: saveSpy, load: vi.fn().mockResolvedValue(undefined) };

    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TABLE_REGISTRY, useValue: [LOCAL_ONLY, NORMAL] },
        { provide: TableServiceFactory, useValue: { getService: () => fakeService } },
        { provide: CacheInvalidationRegistry, useValue: { invalidate: () => {} } },
      ],
    });
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('(a) invalidateAndSync("notifications") KHÔNG xoá store (service.save không được gọi)', async () => {
    const svc = TestBed.inject(MasterDataRegistryService);
    await svc.invalidateAndSync('notifications');
    expect(saveSpy).not.toHaveBeenCalled();
    httpMock.expectNone((r) => r.url.includes('indexeddb/sync'));
  });

  it('(b) syncSingleTable("notifications") KHÔNG gọi API /crm/indexeddb/sync, KHÔNG ghi store', async () => {
    const svc = TestBed.inject(MasterDataRegistryService);
    await svc.syncSingleTable('notifications');
    expect(saveSpy).not.toHaveBeenCalled();
    httpMock.expectNone((r) => r.url.includes('indexeddb/sync'));
  });

  // crm-standalone story-43: BE bump version `business_profile` (không phải bảng IndexedDB, chỉ là tín
  // hiệu reload cho HeaderSyncService) — `save([])` trên store không tồn tại sẽ ném lỗi Dexie.
  it('(c) invalidateAndSync tên NGOÀI registry (`business_profile`) ⇒ no-op, không save, không gọi API', async () => {
    const svc = TestBed.inject(MasterDataRegistryService);
    await svc.invalidateAndSync('business_profile');
    expect(saveSpy).not.toHaveBeenCalled();
    httpMock.expectNone((r) => r.url.includes('indexeddb/sync'));
  });

  it('đối chứng: table THƯỜNG (không localOnly) vẫn đi qua đủ 2 bước xoá + sync', async () => {
    const svc = TestBed.inject(MasterDataRegistryService);
    const promise = svc.invalidateAndSync('units');
    // `service.save([])` chạy đồng bộ trong microtask đầu — chờ 1 tick trước khi flush HTTP.
    await Promise.resolve();
    const req = httpMock.expectOne((r) => r.url.includes('indexeddb/sync?tables=units'));
    req.flush({ units: [{ id: 1 }] });
    await promise;
    expect(saveSpy).toHaveBeenCalled(); // 1 lần clear + 1 lần ghi data mới = 2
    expect(saveSpy.mock.calls.length).toBe(2);
  });
});
