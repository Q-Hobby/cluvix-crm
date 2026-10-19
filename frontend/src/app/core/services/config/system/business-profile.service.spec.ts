import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { firstValueFrom } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { BusinessProfileService } from './business-profile.service';

/**
 * QA story-43 (crm-standalone): BE trả `{ data: <profile> }` (`response.Data` / `response.Updated`).
 * Trước bản vá, `getProfile()`/`updateProfile()` trả nguyên object bọc ⇒ `authBusinessProfile()` mang
 * `{ data }` và mọi consumer (`TimezoneService.tz`, `LocaleFormatService`, header/footer bản in) đọc
 * `undefined` rồi rơi về mặc định. Spec khoá đúng hợp đồng bóc `data`.
 */
describe('BusinessProfileService — bóc `data` khỏi response BE', () => {
  const profile = { name: 'Acme', timezone: 'Asia/Tokyo', currency: 'JPY', updated_at: 5 };

  function setup() {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    return { svc: TestBed.inject(BusinessProfileService), http: TestBed.inject(HttpTestingController) };
  }

  afterEach(() => TestBed.inject(HttpTestingController).verify());

  it('getProfile() → trả profile phẳng, không phải `{ data }`', async () => {
    const { svc, http } = setup();
    const pending = firstValueFrom(svc.getProfile());
    const req = http.expectOne((r) => r.method === 'GET' && r.url.endsWith('config/business-profile'));
    req.flush({ data: profile });
    const got = await pending;
    expect(got.timezone).toBe('Asia/Tokyo');
    expect(got).not.toHaveProperty('data');
  });

  it('updateProfile() → PUT không id trên path, trả profile phẳng', async () => {
    const { svc, http } = setup();
    const pending = firstValueFrom(svc.updateProfile({ name: 'Acme', updated_at: 5 }));
    const req = http.expectOne((r) => r.method === 'PUT' && r.url.endsWith('config/business-profile'));
    req.flush({ data: { ...profile, updated_at: 6 }, message: 'ok' });
    const got = await pending;
    expect(got.updated_at).toBe(6);
    expect(got).not.toHaveProperty('data');
  });
});
