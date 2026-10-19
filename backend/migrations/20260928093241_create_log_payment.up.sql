-- Tạo bảng `log_payment` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `log_payment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `payment_id` int DEFAULT NULL COMMENT 'Thanh toán',
  `action_type` tinyint DEFAULT '2' COMMENT 'Loại hành động: 1: create, 2: update, 3: delete',
  `changed_fields` json DEFAULT NULL COMMENT 'Field thay đổi',
  `changed_by` int DEFAULT NULL,
  `changed_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-log_payment-changed_by` (`changed_by`),
  KEY `fk-log_payment-payment_id` (`payment_id`),
  CONSTRAINT `fk-log_payment-changed_by` FOREIGN KEY (`changed_by`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk-log_payment-payment_id` FOREIGN KEY (`payment_id`) REFERENCES `payment` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
