/**
 * Feature flags FE — wrap các tính năng đang trong giai đoạn rollout / chờ BE.
 *
 * Cách dùng:
 *   import { FEATURE_FLAGS } from '@/define/feature-flags.define';
 *   if (FEATURE_FLAGS.MULTI_SERVICE_EMR) { ... }
 *
 * Production có thể wire vào env var hoặc remote config sau. Hiện tại
 * compile-time const → tree-shake được code chết khi flag = false.
 *
 * Quy tắc:
 * - Flag chỉ tồn tại tạm thời. Khi feature hoàn tất + ổn định → xóa flag.
 * - Không dùng flag để chia branch logic phức tạp lâu dài.
 * - Mỗi flag có doc ghi: mục đích, BE dependency, expected removal date.
 */
export const FEATURE_FLAGS = {
  /**
   * Multi-service episode-based BAĐT.
   * - Cho phép 1 lần khám gắn N BAĐT (BN làm nhiều dịch vụ song song)
   * - BAĐT có record_type='episode' chứa visit_log[] theo từng buổi
   * - Sign theo đợt thay vì theo buổi
   *
   * BE dependency: schema `emr_visit_log`, `appointment_medical_record`,
   * 15 endpoint mới (xem docs/examination_be_api.md mục multi-service).
   *
   * Khi BE ready → set true → QA test → cleanup flag.
   *
   * Spec: docs/multi-service-emr-plan.md
   */
  MULTI_SERVICE_EMR: true,

  /**
   * Gửi SMS thủ công (nút hình máy bay giấy trên breadcrumb → dialog `Sms`).
   *
   * BE dependency: `POST /crm/sms-log/send-sms` hiện là PLACEHOLDER — handler
   * (`internal/modules/webapp/crm/sms_log/handler/handler.go`) chỉ trả 422
   * "Chức năng gửi SMS đang được phát triển", chưa có adapter nhà cung cấp nào
   * trong repo. Kèm theo: `sms_log.customer_id` đang `NOT NULL` + FK tới
   * `customer` nên luồng gửi tự do theo số điện thoại chưa ghi log được —
   * cần migration đổi cột sang nullable khi implement.
   *
   * Tắt để user không bấm vào rồi nhận toast lỗi. Khi BE ready → set true →
   * QA test → cleanup flag. Form FE đã convert sang Signal Forms và sẵn sàng.
   */
  MANUAL_SMS: false,
} as const;

export type FeatureFlag = keyof typeof FEATURE_FLAGS;
