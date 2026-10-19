import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, Input } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { authBusinessProfile, userFullname } from '@/core/services/auth';
import { BusinessProfileModel } from '@/core/models/config/system/business-profile.model';
import { PrintDataService } from './print-data.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-layout-print-footer',
  standalone: true,
  imports: [CommonModule, TranslatePipe],
  template: `
    @if (showThankYou) {
      <div
        class="mt-1"
        [innerHTML]="
          'common.label.print_thankyou'
            | translate
              : {
                  CompanyName: companyInfo().name,
                  Hotline: companyInfo().hotline,
                }
        "
      ></div>
    }
    <div
      [ngClass]="layoutType === 'with-customer-doctor' ? 'pt-[21px]' : 'pt-4'"
      class="mt-2 border-t border-dashed text-lg text-gray-700"
      style="font-size: 14px !important;"
    >
      <div class="flex justify-between text-center">
        @if (showCustomerAndCashier && customerName) {
          <!-- Cột chữ ký khách hàng -->
          <div [ngClass]="columnClass">
            <p class="font-semibold">{{ 'common.label.customer' | translate }}</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16 font-semibold">{{ customerName | titlecase }}</p>
          </div>

          <!-- Cột chữ ký thu ngân -->
          <div [ngClass]="columnClass">
            <p class="font-semibold">{{ 'common.label.cashier' | translate }}</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16">&nbsp;</p>
          </div>
        }

        <!-- Cột chữ ký kế toán trưởng -->
        @if (showAccountant) {
          <div [ngClass]="showCustomerAndCashier && customerName ? 'w-1/3' : 'w-1/2'">
            <p class="font-semibold">Kế toán trưởng</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16">&nbsp;</p>
          </div>
        }

        @if (showAccountant && (!customerName || !showCustomerAndCashier)) {
          <div class="w-1/2">
            <p class="font-semibold">Người lập biểu</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16 font-semibold">{{ userFullname() }}</p>
          </div>
        }

        <!-- Cột chữ ký chung (người lập phiếu / khách hàng) -->
        @if (showDoctor) {
          <div [ngClass]="showCustomerAndCashier && customerName ? columnClass : 'w-2/3'">
            <div class="flex justify-between">
              <div class="w-1/2 text-center">
                <p class="font-semibold">Người lập phiếu</p>
                <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
                <p class="mt-16 font-semibold">{{ userFullname() }}</p>
              </div>

              <div class="w-1/2 text-center">
                <p class="font-semibold">{{ 'common.label.customer' | translate }}</p>
                <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
                <p class="mt-16 font-semibold">{{ customerName | titlecase }}</p>
              </div>
            </div>
          </div>
        }

        <!-- Cột người gửi -->
        @if (showLabo) {
          <div class="w-1/2 text-center">
            <p class="font-semibold">Người gửi</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16 font-semibold">{{ userFullname() }}</p>
          </div>

          <div class="w-1/2 text-center">
            <p class="font-semibold">Người nhận</p>
            <p class="italic text-gray-500">({{ 'common.label.signature' | translate }})</p>
            <p class="mt-16 font-semibold">{{ printData.model()?.doctorName || '&nbsp;' }}</p>
          </div>
        }
      </div>
    </div>
  `,
})
export class PrintFooterLayout {
  @Input() branchId: number | null = null;
  @Input({ required: true }) customerName!: string;
  @Input({
    transform: (
      value: string,
    ): 'default' | 'with-accountant' | 'with-doctor' | 'with-customer-doctor' | 'labo' | 'simple' | 'none' => {
      if (
        value === 'with-accountant' ||
        value === 'with-doctor' ||
        value === 'with-customer-doctor' ||
        value === 'labo' ||
        value === 'simple' ||
        value === 'none'
      ) {
        return value;
      }
      return 'default';
    },
  })
  layoutType: 'default' | 'with-accountant' | 'with-doctor' | 'with-customer-doctor' | 'labo' | 'simple' | 'none' =
    'default';
  protected printData = inject(PrintDataService);
  protected readonly userFullname = userFullname;
  // Hồ sơ doanh nghiệp (thay CompanyHelperService.getCompanyInfo, story-43) — 1 dòng DUY NHẤT.
  companyInfo = computed<Partial<BusinessProfileModel>>(() => authBusinessProfile() ?? {});

  get columnClass(): string {
    return this.layoutType === 'default' ? 'w-1/2' : 'w-1/3';
  }

  get showCustomerAndCashier(): boolean {
    return (
      this.layoutType !== 'with-doctor' &&
      this.layoutType !== 'labo' &&
      this.layoutType !== 'simple' &&
      this.layoutType !== 'none'
    );
  }

  get showAccountant(): boolean {
    return this.layoutType === 'with-accountant';
  }

  get showDoctor(): boolean {
    return this.layoutType === 'with-doctor' || this.layoutType === 'with-customer-doctor';
  }

  get showLabo(): boolean {
    return this.layoutType === 'labo';
  }

  get showThankYou(): boolean {
    return this.layoutType !== 'labo' && this.layoutType !== 'simple';
  }

}
