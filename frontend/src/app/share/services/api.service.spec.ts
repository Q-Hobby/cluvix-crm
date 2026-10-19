import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { afterEach, describe, expect, it } from 'vitest';

import { ApiService } from './api.service';

/**
 * `createHttpOptions` tung hardcode `responseType: 'json'` va im lang vut bo `options.responseType`.
 * Hau qua: `get<Blob>(..., { responseType: 'blob' })` van di duong JSON, Angular dem file .xlsx di
 * `JSON.parse` va nem `Unexpected token 'P'` ("PK" = magic bytes cua zip). Khong test nao bat duoc
 * vi loi chi lo ra luc runtime, o dung endpoint tra file.
 *
 * Test nay neo hanh vi "options cua caller phai toi duoc HttpClient". Mutation check: doi
 * `responseType: options.responseType ?? 'json'` ve `'json'` cung thi 2 `it` dau DO.
 */
describe('ApiService - options cua caller khong duoc bi nuot', () => {
  function setup() {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    return {
      api: TestBed.inject(ApiService),
      http: TestBed.inject(HttpTestingController),
    };
  }

  afterEach(() => TestBed.resetTestingModule());

  it('get() truyen responseType blob xuong HttpClient', () => {
    const { api, http } = setup();
    let received: Blob | null = null;
    api.get<Blob>('config/service/export', { responseType: 'blob' }).subscribe((b) => (received = b));

    const req = http.expectOne((r) => r.url.endsWith('config/service/export'));
    expect(req.request.responseType).toBe('blob');

    // Blob di qua nguyen ven, KHONG bi parse thanh JSON.
    const blob = new Blob(['PK'], { type: 'application/octet-stream' });
    req.flush(blob);
    expect(received).toBe(blob);
    http.verify();
  });

  it('post() truyen responseType text xuong HttpClient', () => {
    const { api, http } = setup();
    api.post<string>('config/service/raw', {}, { responseType: 'text' }).subscribe();
    const req = http.expectOne((r) => r.url.endsWith('config/service/raw'));
    expect(req.request.responseType).toBe('text');
    req.flush('ok');
    http.verify();
  });

  it('mac dinh van la json khi caller khong khai gi', () => {
    const { api, http } = setup();
    api.get('config/service').subscribe();
    const req = http.expectOne((r) => r.url.endsWith('config/service'));
    expect(req.request.responseType).toBe('json');
    req.flush({ data: [] });
    http.verify();
  });

  it('withCredentials theo caller, mac dinh true', () => {
    const { api, http } = setup();
    api.get('config/service').subscribe();
    expect(http.expectOne((r) => r.url.endsWith('config/service')).request.withCredentials).toBe(true);
    http.verify();

    api.get('config/unit', { withCredentials: false }).subscribe();
    expect(http.expectOne((r) => r.url.endsWith('config/unit')).request.withCredentials).toBe(false);
    http.verify();
  });
});
