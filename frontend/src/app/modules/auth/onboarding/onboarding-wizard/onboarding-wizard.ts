import { NgComponentOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

import { authUser, AuthService } from '@/core/services/auth';
import { UserModel } from '@/core/models/hr/user.model';
import { TenantService } from '@/core/services/tenant/tenant.service';
import { ReloadService } from '@/share/services/reload.service';
import { StorageService } from '@/share/services/storage.service';

import { OnboardingBootstrapResult, OnboardingService } from '../onboarding.service';
import { BranchStepValue } from '../steps/branch-step/branch-step';
import { CompanyStepValue } from '../steps/company-step/company-step';
import { OnboardingStepComponent } from '../steps/onboarding-step';
import { ONBOARDING_STEPS } from '../steps/step-registry';

/**
 * QA pass 1 story-08 (P1/P2, `Review Triage Log`) — hàm thuần tách khỏi component để test được KHÔNG
 * cần render (khuôn `resolvePostLoginTarget` ở `syncing.ts`). "Đã xong" = index nhỏ hơn `activeIndex`
 * HOẶC (index KHÁC `activeIndex` VÀ key nằm trong `skipped`) — vế `index !== activeIndex` là fix P1:
 * bản cũ (`index < activeIndex || skipped.has(key)`) không loại trừ bước ĐANG LÀM, nên bước cuối
 * optional bị bấm "Bỏ qua" (submit lỗi, `error` callback chỉ reset `isSubmitting`, KHÔNG xoá
 * `skipped` cũng KHÔNG đổi `activeIndex`) sẽ hiện `pi-check` như đã xong dù đang đứng ngay tại đó.
 */
export function resolveStepDone(
  index: number,
  activeIndex: number,
  stepKey: string,
  skipped: ReadonlySet<string>,
): boolean {
  return index !== activeIndex && (index < activeIndex || skipped.has(stepKey));
}

/**
 * Shell wizard onboarding — chỉ biết `OnboardingStepDef`/`OnboardingStepComponent` (contract), KHÔNG
 * biết field nghiệp vụ bên trong từng bước. Thêm/bớt bước ⇒ sửa `step-registry.ts` (AC4).
 *
 * Story-08 AC7 — thay khung `p-stepper` (header-only, chốt ở story-04) bằng layout side-by-side theo
 * yêu cầu user: cột trái tự vẽ danh sách bước (vòng tròn trạng thái + tiêu đề/mô tả), cột phải render
 * nội dung bước bằng `NgComponentOutlet` (giữ nguyên từ story-04 — component nội dung đổi ĐỘNG theo
 * registry nên không dùng `p-step-panels`/`p-step-panel`, panel PrimeNG ép nội dung theo template
 * tĩnh per-step). Điều hướng (Quay lại/Bỏ qua/Tiếp/Hoàn tất) hoàn toàn tự viết, KHÔNG đổi.
 *
 * Lấy instance bước đang render để gọi `valid()/markTouched()/value()`: `NgComponentOutlet` có sẵn
 * getter `componentInstance` (public API `@angular/common`, export `ngComponentOutlet` làm template
 * ref) — dùng thẳng, KHÔNG cần `ViewContainerRef.createComponent()` thủ công.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  selector: 'app-onboarding-wizard',
  templateUrl: './onboarding-wizard.html',
  imports: [TranslatePipe, ButtonModule, NgComponentOutlet],
})
export class OnboardingWizard {
  private readonly onboardingService = inject(OnboardingService);
  private readonly authService = inject(AuthService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly storageService = inject(StorageService);
  private readonly reloadService = inject(ReloadService);
  private readonly tenantService = inject(TenantService);

  protected readonly steps = ONBOARDING_STEPS;

  protected readonly activeIndex = signal(0);
  /** `key` → giá trị bước đã lưu (AC6: quay lại rồi tiến tiếp phải nhận lại đúng giá trị này). */
  protected readonly draft = signal<Record<string, unknown>>({});
  /** `key` các bước user đã bấm "Bỏ qua" — quay lại nhập rồi tiến tiếp thì bị gỡ khỏi set này. */
  protected readonly skipped = signal<Set<string>>(new Set());
  protected readonly isSubmitting = signal(false);

  protected readonly currentStep = computed(() => this.steps[this.activeIndex()]);
  protected readonly isLastStep = computed(() => this.activeIndex() === this.steps.length - 1);
  protected readonly stepInputs = computed(() => ({
    initialValue: this.draft()[this.currentStep().key],
  }));

  private readonly outletRef = viewChild<NgComponentOutlet>('outlet');

  private get stepInstance(): OnboardingStepComponent | null {
    return (this.outletRef()?.componentInstance as OnboardingStepComponent | null) ?? null;
  }

  /** Story-08 AC7 — trạng thái "đã xong" cho vòng tròn cột trái; logic thật ở `resolveStepDone()`. */
  protected isStepDone(index: number): boolean {
    return resolveStepDone(index, this.activeIndex(), this.steps[index].key, this.skipped());
  }

  protected back(): void {
    if (this.isSubmitting() || this.activeIndex() === 0) return;
    this.activeIndex.update((i) => i - 1);
  }

  /**
   * QA pass 2 ac_gap (`Story Change Log` AC5b, user chốt phương án A): bước `optional` mà THẬT SỰ
   * TRỐNG (`isEmpty()`) ⇒ coi như bỏ qua, cho "Tiếp" đi tiếp. Bước `optional` mà user đã gõ DỞ (không
   * trống nhưng `!valid()`) ⇒ CHẶN như bước bắt buộc — KHÔNG được âm thầm bỏ dữ liệu đã nhập (đây
   * chính là ac_gap pass 2 tìm ra ở fix pass 1: fix cũ coi mọi `!valid()` trên bước optional là "bỏ
   * qua", kể cả khi user đã gõ một phần).
   */
  protected next(): void {
    if (this.isSubmitting()) return;
    const instance = this.stepInstance;
    if (!instance) return;
    const step = this.currentStep();
    if (step.optional && instance.isEmpty()) {
      this.markStepSkipped(step.key);
      this.advance();
      return;
    }
    if (!instance.valid()) {
      instance.markTouched();
      return;
    }
    this.commitCurrentStep(instance);
    this.advance();
  }

  /**
   * `skip()` = bỏ qua TƯỜNG MINH (AC5b): luôn xoá `draft[key]` (dù đang có dữ liệu dở hay không — bấm
   * "Bỏ qua" là quyết định rõ ràng "không dùng dữ liệu bước này") rồi đánh dấu `skipped`. Trên bước
   * KHÔNG PHẢI bước cuối: `advance()` như cũ. Trên bước CUỐI (QA finding P-high pass 1, giữ nguyên
   * quyết định): không có bước nào sau để advance tới, và bấm "Bỏ qua" ở đây rõ ràng muốn kết thúc
   * luôn với bước cuối bị bỏ qua — chọn "Bỏ qua = bỏ qua + hoàn tất" thay vì ẩn nút "Bỏ qua" ở bước
   * cuối (ẩn nút buộc user quay lại suy luận hiệu ứng tương đương qua `finish()` — thừa thao tác).
   */
  protected skip(): void {
    if (this.isSubmitting()) return;
    this.markStepSkipped(this.currentStep().key);
    if (this.isLastStep()) {
      this.submit();
    } else {
      this.advance();
    }
  }

  /**
   * Bước cuối cùng: validate + commit như `next()` rồi gọi `onboardingService.bootstrap`.
   *
   * QA pass 3 story_defect (SỬA CÂU CHỮ AC5b — xem `Story Change Log`): bản pass 2 từng có thêm vế
   * `|| this.skipped().has(step.key)` vào điều kiện auto-skip — TỰ NÓ là bug data-loss mới: bước
   * từng bị `skip()` (kể cả submit lỗi rồi quay lại) mà `skipped` vẫn còn key thì `finish()` coi là
   * "vẫn bỏ qua" VĨNH VIỄN, kể cả khi user đã quay lại nhập dữ liệu MỚI HOÀN TOÀN HỢP LỆ — nhánh
   * `commitCurrentStep()` phía dưới không bao giờ chạy tới vì auto-skip return sớm. Đã BỎ vế đó:
   * điều kiện auto-skip giờ CHỈ còn `step.optional && instance.isEmpty()` (khớp y hệt `next()`).
   * Trạng thái "đã skip" không còn là nhãn dán vĩnh viễn — mỗi lần `finish()`/`next()` đọc lại
   * `isEmpty()` THẬT của instance đang render; `commitCurrentStep()` (chạy khi bước KHÔNG trống) đã tự
   * gỡ key khỏi `skipped` khi commit thành công (logic có sẵn từ pass 1, không cần sửa gì thêm).
   *
   * Phân biệt 3 nhánh: (1) optional + `isEmpty()` THẬT SỰ trống ⇒ coi là bỏ qua, submit ngay — KHÔNG
   * ghi `value()` vào draft; (2) optional nhưng có nhập mà vẫn `!valid()` (nhập dở) ⇒ `markTouched()`
   * + CHẶN như bước bắt buộc, không cho hoàn tất; (3) hợp lệ (bắt buộc hoặc optional) ⇒
   * `commitCurrentStep()` + submit (tự gỡ `skipped` nếu trước đó đã skip).
   *
   * Story-06: `buildBootstrapFormData()` dựng payload THẬT (`dto.CompanyInput`/`dto.BranchInput`);
   * thành công ⇒ đuôi xử lý đúng khuôn `choose-company.ts:164-195` (cập nhật `authUser`, điều hướng).
   * Lỗi ⇒ interceptor đã toast (không tự thêm toast — B.5), chỉ mở khoá nút, giữ nguyên draft để user
   * sửa lại (AC5).
   */
  protected finish(): void {
    if (this.isSubmitting()) return;
    const instance = this.stepInstance;
    if (!instance) return;
    const step = this.currentStep();

    if (step.optional && instance.isEmpty()) {
      this.markStepSkipped(step.key);
      this.submit();
      return;
    }
    if (!instance.valid()) {
      instance.markTouched();
      return;
    }
    this.commitCurrentStep(instance);
    this.submit();
  }

  /** Dùng chung cho `skip()`/`next()`/`finish()` khi coi 1 bước là "đã bỏ qua" — replace reference. */
  private markStepSkipped(key: string): void {
    this.skipped.update((s) => new Set(s).add(key));
    this.draft.update((d) => {
      if (!(key in d)) return d;
      const next = { ...d };
      delete next[key];
      return next;
    });
  }

  /**
   * AC4/AC6 — double-click guard: `isSubmitting` chặn NGAY từ `next()/skip()/finish()` (đã set
   * `true` ở đây trước khi request đi), không phải chặn ở `submit()` — click thứ 2 trong lúc request
   * đang bay bị các hàm gọi vào (`next/skip/finish`) tự return sớm ở dòng `if (this.isSubmitting())
   * return;`, `submit()` không bao giờ chạy 2 lần.
   *
   * Thành công: đúng thứ tự `choose-company.ts:164-195` — cập nhật `authUser` (object MỚI, không
   * mutate) → `storageService.saveStorage('auth_user', …)` → `reloadService.sendMessage('reload')` →
   * `await tenantService.dispatchTenantChange()` → `window.location.href = '/'` (KHÔNG
   * `location.reload()` — cùng lý do `choose-company.ts:201-209`: trang này CHÍNH LÀ trang chặn,
   * reload tại chỗ ném user về lại `/choose-company`). KHÔNG `finalize()` mở khoá khi thành công
   * (sẽ nhấp nháy nút trước khi `window.location.href` điều hướng đi) — chỉ mở khoá ở nhánh `error`.
   */
  private submit(): void {
    this.isSubmitting.set(true);
    this.onboardingService
      .bootstrap(this.buildBootstrapFormData())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: async (res: OnboardingBootstrapResult) => {
          try {
            const current = authUser();
            if (current) {
              const branchId =
                res.branch_id != null
                  ? current.branchId?.includes(res.branch_id)
                    ? current.branchId
                    : [...(current.branchId ?? []), res.branch_id]
                  : current.branchId;
              const updated: UserModel = {
                ...current,
                branchId,
                userProfile: current.userProfile
                  ? {
                      ...current.userProfile,
                      branchActive_id: res.branch_id ?? undefined,
                    }
                  : current.userProfile,
              };
              authUser.set(updated);
              this.storageService.saveStorage('auth_user', updated);
            }
            // Bootstrap vừa tạo hồ sơ doanh nghiệp (company step) — nạp lại authBusinessProfile
            // trước khi reload để print header/currency pipe không còn đọc giá trị mặc định cũ.
            await this.authService.refreshBusinessProfile();
            this.reloadService.sendMessage('reload');
            await this.tenantService.dispatchTenantChange();
          } catch (err) {
            console.error('[OnboardingWizard] async subscribe error:', err);
          } finally {
            window.location.href = '/';
          }
        },
        // Interceptor chung đã toast lỗi (B.5) — chỉ mở khoá nút, draft giữ nguyên (AC5).
        error: () => this.isSubmitting.set(false),
      });
  }

  private commitCurrentStep(instance: OnboardingStepComponent): void {
    const key = this.currentStep().key;
    this.draft.update((d) => ({ ...d, [key]: instance.value() }));
    if (this.skipped().has(key)) {
      this.skipped.update((s) => {
        const next = new Set(s);
        next.delete(key);
        return next;
      });
    }
  }

  private advance(): void {
    if (this.isLastStep()) return;
    this.activeIndex.update((i) => i + 1);
  }

  /**
   * AC3 — `company` LUÔN gửi (bước bắt buộc, đã `commitCurrentStep()` trước khi tới đây); `branch`
   * CHỈ gửi khi KHÔNG nằm trong `skipped` — kiểm `skipped` là nguồn sự thật, KHÔNG suy diễn từ
   * "`draft['branch']` có tồn tại hay không" (đúng bug class mutation hay gặp: gửi `branch` dù bước đã
   * bị bỏ qua). `markStepSkipped()` cũng xoá `draft[key]` nên bình thường 2 điều kiện trùng nhau —
   * nhưng gate chính thức PHẢI là `skipped`, phòng trường hợp `draft` còn giữ giá trị cũ.
   *
   * KHÔNG dùng `toFormData()` chung (tách object lồng thành `company[name]`) — BE story-02 đọc JSON
   * string nguyên khối qua `dto.CompanyInput`/`dto.BranchInput`.
   */
  private buildBootstrapFormData(): FormData {
    const fd = new FormData();

    const company = this.draft()['company'] as CompanyStepValue | undefined;
    if (company) {
      const { logoFile, ...companyJson } = company;
      fd.append('company', JSON.stringify(companyJson));
      if (logoFile) fd.append('logoFile', logoFile);
    }

    if (!this.skipped().has('branch')) {
      const branch = this.draft()['branch'] as BranchStepValue | undefined;
      if (branch) {
        fd.append('branch', JSON.stringify(branch));
      }
    }

    return fd;
  }
}
