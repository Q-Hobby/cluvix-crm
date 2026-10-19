-- Tạo bảng `payment_method` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `payment_method` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái hoạt động',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-payment_method-created_by` (`created_by`),
  KEY `fk-payment_method-updated_by` (`updated_by`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
