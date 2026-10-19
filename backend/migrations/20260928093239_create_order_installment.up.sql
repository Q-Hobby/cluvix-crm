-- Tạo bảng `order_installment` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `order_installment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `order_service_used_id` int NOT NULL COMMENT 'ID của order_service_used',
  `period_number` int NOT NULL COMMENT 'Kỳ thứ mấy (1,2,3...)',
  `amount` decimal(20,2) NOT NULL COMMENT 'Số tiền kỳ này',
  `due_date` date NOT NULL COMMENT 'Ngày đến hạn',
  `paid_date` date DEFAULT NULL COMMENT 'Ngày thanh toán thực tế',
  `paid_amount` decimal(20,2) DEFAULT '0.00' COMMENT 'Số tiền đã thanh toán',
  `status` tinyint DEFAULT '1' COMMENT '1: Chưa thu, 2: Đã thu, 3: Quá hạn, 4: Thanh toán 1 phần',
  `note` text COMMENT 'Ghi chú',
  `created_at` bigint DEFAULT NULL COMMENT 'Ngày tạo',
  PRIMARY KEY (`id`),
  KEY `idx-order_installment-due_date` (`due_date`),
  KEY `idx-order_installment-order_service_used_id` (`order_service_used_id`),
  KEY `idx_oi_status_due_cover` (`status`,`due_date`,`order_service_used_id`,`amount`,`paid_amount`),
  CONSTRAINT `fk-order_installment-order_service_used_id` FOREIGN KEY (`order_service_used_id`) REFERENCES `order_service_used` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
