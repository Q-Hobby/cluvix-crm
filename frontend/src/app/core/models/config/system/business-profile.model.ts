import { Identifiable } from '@/core/interface/identifiable';

/**
 * Hồ sơ doanh nghiệp — bảng `business_profile` (BE story-63), 1 dòng DUY NHẤT (CRM standalone,
 * DB-per-customer — không còn khái niệm nhiều "company" trong 1 DB). Thay `CompanyModel` (đã xoá,
 * story-43) làm nguồn thông tin thương hiệu/timezone/tiền tệ/2FA/IP allowlist cho toàn app.
 *
 * `GET /api/config/business-profile` trả cấu trúc này (`business_profile/dao.SelectCols`, BE).
 */
export interface BusinessProfileModel extends Identifiable {
  id?: number;
  name?: string;
  code?: string;
  hotline?: string;
  address?: string;
  email?: string;
  website?: string;
  logo_image?: string | null;
  tax_code?: string;
  timezone?: string;
  currency?: string;
  currency_symbol?: string;
  currency_locale?: string;
  /**
   * FE-side: mảng weekday số (0=CN..6=T7) cho binding trực tiếp `p-select multiple`. BE lưu CSV
   * string (`weekly_rest_days` cột DB) — convert CSV<->mảng ở boundary trong `BusinessProfileForm`,
   * KHÔNG phải type BE thật (cùng quy ước `CompanyModel` cũ).
   */
  weekly_rest_days?: number[];
  /**
   * FE-side: text nhiều dòng (mỗi dòng 1 IP/CIDR) cho binding trực tiếp `<textarea pTextarea>`. BE
   * lưu JSON array string[] (`allowed_ips` cột JSON) — convert JSON<->text ở boundary trong
   * `BusinessProfileForm`, KHÔNG phải type BE thật.
   */
  allowed_ips?: string;
  /** 1=bắt buộc 2FA toàn hệ thống, 0=không — bind trực tiếp `p-toggleswitch [trueValue]="1" [falseValue]="0"`. */
  require_2fa?: number;
  /** List role bắt buộc 2FA (BE JSON array string[], khớp thẳng type FE không cần convert). */
  require_2fa_roles?: string[];
  /** Số ngày tin thiết bị sau xác thực 2FA (0 = luôn hỏi lại). */
  totp_trust_days?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
}
