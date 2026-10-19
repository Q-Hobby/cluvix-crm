import { ChangeDetectionStrategy, Component, computed, effect, EventEmitter, inject, input, OnInit, Output } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { authBusinessProfile } from '@/core/services/auth';
import { BusinessProfileModel } from '@/core/models/config/system/business-profile.model';
import { BranchHelperService } from '@/indexeddb/config/components/branch';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-layout-print-header',
  standalone: true,
  template: `
    <div
      class="sheet_head border-b pb-1"
      style="page-break-inside: avoid; border-bottom: 1px solid #000 !important;"
    >
      <div
        class="flex items-center gap-4"
        style="align-items: center !important; display: flex !important;"
      >
        <!-- Logo: co giãn tự nhiên, không chiếm chỗ khi không có -->
        @if (companyInfo().logo_image) {
          <img
            [src]="companyInfo().logo_image"
            [alt]="companyInfo().name"
            class="flex-none max-h-[72px] object-contain"
            style="flex: none !important; max-height: 72px !important;"
            fetchpriority="high"
            (load)="onLogoLoaded()"
            (error)="onLogoLoaded()"
          />
        }
        <!-- Thông tin công ty/chi nhánh -->
        <div class="flex-1 leading-tight" style="flex: 1 !important;">
          <h3 class="font-bold uppercase">
            {{ companyInfo().name }}
          </h3>

          @if (displayAddress()) {
            <p class="break-words">
              {{ 'common.label.address' | translate }}:
              {{ displayAddress() }}
            </p>
          }
          @if (displayHotline()) {
            <p>Hotline: {{ displayHotline() }}</p>
          }
          @if (companyInfo().tax_code) {
            <p>{{ 'config.company.tax_code' | translate }}: {{ companyInfo().tax_code }}</p>
          }
          <p>
            @if (displayEmail()) {
              <span>Email: {{ displayEmail() }}</span>
            }
            @if (displayEmail() && companyInfo().website) {
              <span> | </span>
            }
            @if (companyInfo().website) {
              <span>Website: {{ companyInfo().website }}</span>
            }
          </p>
        </div>
      </div>
    </div>
  `,
  imports: [TranslatePipe],
})
export class PrintHeaderLayout implements OnInit {
  /** `null` ⇒ trang in không gắn 1 chi nhánh — địa chỉ/hotline/email lấy từ hồ sơ doanh nghiệp. */
  branchId = input<number | null>(null);
  @Output() logoLoaded = new EventEmitter<void>();
  displayAddress = computed(() => this.branchInfo()?.address || this.companyInfo()?.address || '');
  displayHotline = computed(() => this.branchInfo()?.hotline || this.companyInfo()?.hotline || '');
  displayEmail = computed(() => this.branchInfo()?.email || this.companyInfo()?.email || '');
  // Hồ sơ doanh nghiệp (thay CompanyHelperService.getCompanyInfo, story-43) — 1 dòng DUY NHẤT.
  companyInfo = computed<Partial<BusinessProfileModel>>(() => authBusinessProfile() ?? {});
  private branchHelper: BranchHelperService = inject(BranchHelperService);
  branchInfo = computed(() => {
    return this.branchHelper.getBranchInfo(this.branchId());
  });

  constructor() {
    // Preload logo ngay khi URL available để trình duyệt tải ảnh sớm nhất có thể
    // Không cần chờ <img> tag render mới bắt đầu fetch
    effect(() => {
      const logoUrl = this.companyInfo()?.logo_image;
      if (logoUrl) {
        this.preloadLogo(logoUrl);
      }
    });
  }

  async ngOnInit() {
    await this.branchHelper.ensureBranchesLoaded();

    // Nếu không có logo thì emit ngay
    if (!this.companyInfo()?.logo_image) {
      setTimeout(() => this.onLogoLoaded(), 0);
    }
  }

  onLogoLoaded() {
    this.logoLoaded.emit();
  }

  // Inject <link rel="preload"> vào head để browser fetch logo song song trước khi img render
  private preloadLogo(url: string): void {
    const id = 'print-logo-preload';
    if (document.getElementById(id)) return; // đã preload rồi
    const link = document.createElement('link');
    link.id = id;
    link.rel = 'preload';
    link.as = 'image';
    link.href = url;
    document.head.appendChild(link);
  }
}
