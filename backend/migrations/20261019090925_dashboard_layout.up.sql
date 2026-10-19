-- Dashboard CRM tuỳ biến (port từ ERP epic crm-dashboard-customizable story-04): bố cục khối dashboard.
-- 1 dòng / (user, dashboard). user_id NULL = mặc định THƯƠNG HIỆU (người có manageConfig đặt).
-- 1 DB = 1 thương hiệu (D15) ⇒ KHÔNG có company_id; UNIQUE chỉ cần đúng trong DB này.
-- UNIQUE dùng functional key part COALESCE(user_id, 0): UNIQUE trên cột NULL cho phép trùng (NULL ≠ NULL);
-- KHÔNG dùng cột sinh STORED vì MySQL từ chối FK trên cột nền của cột sinh STORED.
-- user_id: FK thật (quan hệ sở hữu, không phải audit) — xoá user ⇒ CASCADE dọn bố cục của user đó.
--   Chi phí: thêm 1 FK con cho bảng `user` (~5ms/FK con mỗi UPDATE user, xem CLAUDE.md mục FK) — chấp
--   nhận vì thiếu FK thì xoá user để lại bố cục mồ côi và DeleteWithCheck không bảo vệ được gì.
-- created_by/updated_by: cột audit ⇒ KHÔNG FK (Hard Rule ngoại lệ), chỉ KEY.
-- Idempotent: CREATE TABLE IF NOT EXISTS (chạy lại khi dirty state không lỗi).
CREATE TABLE IF NOT EXISTS `dashboard_layout` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL COMMENT 'Chủ bố cục; NULL = mặc định thương hiệu',
  `dashboard_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Dashboard (vd crm)',
  `config` json NOT NULL COMMENT 'Bố cục {version, items[{key, zone, visible, size}]}',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint NOT NULL DEFAULT '0',
  `created_by` int DEFAULT NULL,
  `updated_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_dashboard_layout_owner` ((COALESCE(`user_id`, 0)), `dashboard_key`),
  KEY `idx_dashboard_layout_user` (`user_id`),
  KEY `idx_dashboard_layout_created_by` (`created_by`),
  KEY `idx_dashboard_layout_updated_by` (`updated_by`),
  CONSTRAINT `fk_dashboard_layout_user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Bố cục dashboard tuỳ biến';
