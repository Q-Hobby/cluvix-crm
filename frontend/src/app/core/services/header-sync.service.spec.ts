import { TestBed } from '@angular/core/testing';
import { HttpHeaders, provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { Router } from '@angular/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { HeaderSyncService } from './header-sync.service';
import { TokenService } from '@/core/services/auth/token.service';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { BaseIndexedDBService } from '@/indexeddb/config/service/base-indexed-db.service';

/**
 * crm-standalone story-43 (QA): BE `config/business_profile` bump version `business_profile` (thay
 * `companies` đã bỏ ở story-62). Một client KHÁC (IndexedDB riêng, chưa có version đó) nhận header
 * `X-Dexie-Versions: business_profile=<mới>` PHẢI rơi vào nhánh reload (nạp lại hồ sơ: timezone,
 * tiền tệ, logo) — trước bản vá map reload còn khoá `companies` nên không ai reload.
 *
 * `location.reload()` chạy trong `setTimeout` — dùng fake timer và KHÔNG tua để jsdom không điều
 * hướng; bằng chứng đã chọn nhánh reload là `sessionStorage.post_reload_toast`.
 */
describe('HeaderSyncService — đổi hồ sơ doanh nghiệp ⇒ client khác reload', () => {
  let invalidateAndSync: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    sessionStorage.clear();
    invalidateAndSync = vi.fn().mockResolvedValue(undefined);
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TokenService, useValue: { getAccessToken: () => 'tok' } },
        { provide: Router, useValue: { url: '/customer' } },
        { provide: MasterDataRegistryService, useValue: { invalidateAndSync } },
        {
          provide: BaseIndexedDBService,
          useValue: {
            getLocalVersion: () => Promise.resolve(1), // đã full sync trước đó
            getTableVersions: () => Promise.resolve({ branches: 10 }),
            setTableVersions: () => Promise.resolve(),
          },
        },
      ],
    });
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    sessionStorage.clear();
  });

  const headers = (v: string) => new HttpHeaders({ 'X-Dexie-Versions': v });

  it('`business_profile` mới ⇒ chọn nhánh reload kèm toast thương hiệu', async () => {
    const svc = TestBed.inject(HeaderSyncService);
    await svc.checkVersionFromHeaders(headers('branches=10,business_profile=99'));
    expect(sessionStorage.getItem('post_reload_toast')).toBe('Đã cập nhật thông tin thương hiệu');
    expect(invalidateAndSync).toHaveBeenCalledWith('business_profile');
  });

  it('đối chứng: version không đổi ⇒ KHÔNG reload', async () => {
    const svc = TestBed.inject(HeaderSyncService);
    await svc.checkVersionFromHeaders(headers('branches=10'));
    expect(sessionStorage.getItem('post_reload_toast')).toBeNull();
    expect(invalidateAndSync).not.toHaveBeenCalled();
  });
});
