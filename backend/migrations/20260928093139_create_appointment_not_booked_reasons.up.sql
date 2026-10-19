-- Tạo bảng `appointment_not_booked_reasons` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `appointment_not_booked_reasons` (
  `id` int NOT NULL AUTO_INCREMENT,
  `parent_id` int DEFAULT NULL COMMENT 'Danh mục cha',
  `name` varchar(255) NOT NULL COMMENT 'Tên',
  `desc` text COMMENT 'Mô tả',
  `status` int DEFAULT '1' COMMENT 'Trạng thái hiển thị',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `updated_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-appointment_not_booked_reasons-created_by` (`created_by`),
  KEY `fk-appointment_not_booked_reasons-updated_by` (`updated_by`),
  KEY `fk_appointment_not_booked_reason-parent_id` (`parent_id`),
  CONSTRAINT `fk_appointment_not_booked_reason-parent_id` FOREIGN KEY (`parent_id`) REFERENCES `appointment_not_booked_reasons` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
