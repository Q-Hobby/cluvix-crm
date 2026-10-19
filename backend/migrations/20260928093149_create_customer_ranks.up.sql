-- Tạo bảng `customer_ranks` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `customer_ranks` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL COMMENT 'Tên hạng (VIP, Gold, Silver, Bronze)',
  `min_points` int NOT NULL DEFAULT '0' COMMENT 'Điểm tối thiểu để đạt hạng',
  `max_points` int DEFAULT NULL COMMENT 'Điểm tối đa (NULL = không giới hạn)',
  `discount_percent` decimal(5,2) DEFAULT '0.00' COMMENT '% giảm giá chung (chỉ áp dụng khi không có dịch vụ riêng)',
  `priority_level` int DEFAULT '0' COMMENT 'Mức ưu tiên đặt lịch',
  `color_code` varchar(50) DEFAULT NULL COMMENT 'Màu hiển thị (#FF0000 hoặc tên màu)',
  `description` text COMMENT 'Mô tả quyền lợi',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái hoạt động',
  `created_at` bigint DEFAULT NULL COMMENT 'Ngày tạo',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
