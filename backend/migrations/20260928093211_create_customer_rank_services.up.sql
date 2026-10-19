-- Tạo bảng `customer_rank_services` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `customer_rank_services` (
  `id` int NOT NULL AUTO_INCREMENT,
  `rank_id` int NOT NULL COMMENT 'ID hạng thành viên',
  `service_ids` json DEFAULT NULL COMMENT 'Danh sách ID dịch vụ (JSON array)',
  `discount_percent` decimal(5,2) DEFAULT '0.00' COMMENT '% giảm giá riêng cho các dịch vụ này',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái hoạt động',
  `created_at` bigint DEFAULT NULL COMMENT 'Ngày tạo',
  PRIMARY KEY (`id`),
  KEY `idx-customer_rank_services-rank_id` (`rank_id`),
  CONSTRAINT `fk-customer_rank_services-rank_id` FOREIGN KEY (`rank_id`) REFERENCES `customer_ranks` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
