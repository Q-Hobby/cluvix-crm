import { OnboardingBranchStep } from './branch-step/branch-step';
import { OnboardingCompanyStep } from './company-step/company-step';
import { OnboardingStepDef } from './onboarding-step';

/**
 * Nguồn DUY NHẤT khai báo các bước wizard onboarding. Shell (`onboarding-wizard.ts`) chỉ đọc mảng
 * này — thêm/bớt/đổi thứ tự bước CHỈ sửa ở đây (AC4).
 *
 * Story-04 dùng 2 bước PLACEHOLDER để chứng minh khung (đã XOÁ, AC8). Story-05 (bước `company`) đã
 * thay bằng `company-step` thật; story-06 (bước `branch`) thay nốt bằng `branch-step` thật.
 *
 * `branch.optional` đổi `true` → `false` (2026-09-19, đảo AC6 gốc): "bỏ qua chi nhánh" từng cho
 * company tạo ra mà KHÔNG chi nhánh nào ⇒ BE không seed được `config_code` (không có branch để
 * gắn) ⇒ company đó không sinh nổi mã dịch vụ đầu tiên (sự cố thật ARA Plus). BE
 * (`config/onboarding/service/service.go` `Validate`) giờ từ chối payload thiếu branch — đổi ở
 * đây để FE khớp, KHÔNG cho user đi tới màn hình chặn ở BE. Nút "Bỏ qua" (`skip()`,
 * `onboarding-wizard.html`) tự ẩn khi `optional=false`, không cần sửa gì thêm ở shell.
 */
export const ONBOARDING_STEPS: OnboardingStepDef[] = [
  {
    key: 'company',
    titleKey: 'common.label.company',
    descKey: 'auth.onboarding.step_company_desc',
    optional: false,
    component: OnboardingCompanyStep,
  },
  {
    key: 'branch',
    titleKey: 'common.label.branch',
    descKey: 'auth.onboarding.step_branch_desc',
    optional: false,
    component: OnboardingBranchStep,
  },
];
