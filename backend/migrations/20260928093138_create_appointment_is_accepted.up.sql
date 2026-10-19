-- Tạo bảng `appointment_is_accepted` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `appointment_is_accepted` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Trạng thái tư vấn',
  `severity` varchar(25) DEFAULT 'info' COMMENT 'Màu',
  `desc` text COMMENT 'Mô tả',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Trạng thái',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  `isAccepted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '1 = trạng thái này tính là đồng ý/chốt (dùng để tính tỉ lệ chốt, doanh số, TB bill)',
  PRIMARY KEY (`id`),
  KEY `fk-appointment_is_accepted-created_by-user-id` (`created_by`),
  KEY `fk-appointment_is_accepted-updated_by-user-id` (`updated_by`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
