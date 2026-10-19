-- Tạo bảng `card_type` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `card_type` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên',
  `method` int NOT NULL COMMENT 'Áp dụng cho hình thức thanh toán nào',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái hoạt động, mặc định: 1',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí hiển thị',
  `note` text COMMENT 'Ghi chú',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-card_type-created_by-user-id` (`created_by`),
  KEY `fk-card_type-updated_by-user-id` (`updated_by`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
