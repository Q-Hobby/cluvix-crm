import { describe, expect, it } from 'vitest';

import { resolvePostLoginTarget } from './syncing';

/**
 * story-07 (onboarding-setup-wizard) AC7 — hàm THUẦN quyết định đích sau /syncing.
 * story-42 (E6 bỏ company): không còn màn ép chọn thương hiệu — login flow luôn đi thẳng vào
 * `redirect`, không còn nhánh theo company active.
 * Chạy: `npx vitest run -c vitest-base.config.ts src/app/modules/auth/syncing/syncing.spec.ts`
 */
describe('resolvePostLoginTarget', () => {
  it('(a) redirect mặc định "/" ⇒ "/"', () => {
    expect(resolvePostLoginTarget('/')).toBe('/');
  });

  it('(b) redirect deep-link ⇒ giữ nguyên redirect', () => {
    expect(resolvePostLoginTarget('/customer')).toBe('/customer');
  });
});
