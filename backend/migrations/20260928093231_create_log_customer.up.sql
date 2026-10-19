-- Tạo bảng `log_customer` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `log_customer` (
  `id` int NOT NULL AUTO_INCREMENT,
  `customer_id` int DEFAULT NULL COMMENT 'Khách hàng',
  `action_type` tinyint DEFAULT '2' COMMENT 'Loại hành động: 1: create, 2: update, 3: delete',
  `changed_fields` json DEFAULT NULL COMMENT 'Field thay đổi',
  `changed_by` int DEFAULT NULL,
  `changed_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-log_customer-changed_by` (`changed_by`),
  KEY `fk-log_customer-customer_id` (`customer_id`),
  CONSTRAINT `fk-log_customer-changed_by` FOREIGN KEY (`changed_by`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk-log_customer-customer_id` FOREIGN KEY (`customer_id`) REFERENCES `customer` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
