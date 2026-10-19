import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';

import { AuthService } from '@/core/services/auth/auth.service';
import { safeInternalRedirect } from '@/core/services/auth/safe-redirect';
import { HeaderSyncService } from '@/core/services/header-sync.service';
import { BrandLoader } from '@/share/components/brand-loader/brand-loader';

/**
 * Hàm THUẦN quyết định đích điều hướng sau /syncing — tách riêng để test không cần render
 * component (AC7). story-42 (E6 bỏ company): không còn màn ép chọn thương hiệu — login flow đi
 * thẳng vào `redirect` (mặc định `/`, hoặc deep-link `?redirect=`).
 */
export function resolvePostLoginTarget(redirect: string): string {
  return redirect;
}

@Component({
  template: `
    <style>
      .syncing-container {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        background: #7b3f9e;
      }
    </style>
    <div class="syncing-container">
      <app-brand-loader color="#fff" textColor="#fff" [text]="'auth.login.syncing' | translate" />
    </div>
  `,
  standalone: true,
  imports: [TranslatePipe, BrandLoader],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Syncing {
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  private headerSync = inject(HeaderSyncService);
  private authService = inject(AuthService);

  async ngOnInit() {
    const redirect = safeInternalRedirect(this.route.snapshot.queryParamMap.get('redirect')) || '/';

    // Thứ tự giữ nguyên (AC3): sync IndexedDB rồi mới refresh hồ sơ doanh nghiệp — `authBusinessProfile`
    // (currency pipe/print header, story-43) không phụ thuộc IndexedDB nhưng giữ nguyên thứ tự await
    // cũ để không đổi hành vi ngoài phạm vi story-42.
    await this.headerSync.ensureSyncedIfEmpty();
    await this.authService.refreshBusinessProfile();

    const target = resolvePostLoginTarget(redirect);

    await this.router.navigateByUrl(target, { replaceUrl: true });
  }
}
