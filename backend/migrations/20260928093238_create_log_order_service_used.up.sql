-- Tạo bảng `log_order_service_used` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `log_order_service_used` (
  `id` int NOT NULL AUTO_INCREMENT,
  `order_service_used_id` int DEFAULT NULL COMMENT 'FK → order_service_used.id, NULL nếu dòng dịch vụ đã bị xoá',
  `action_type` tinyint NOT NULL DEFAULT '2' COMMENT '1=create 2=update 3=delete',
  `changed_fields` json DEFAULT NULL COMMENT 'Diff {field: {from, to}}',
  `changed_by` int DEFAULT NULL COMMENT 'FK → user.id',
  `changed_at` int NOT NULL COMMENT 'Unix timestamp',
  `reason` varchar(500) DEFAULT NULL COMMENT 'Ly do thay doi (free-text, optional) - dung cho luong doi dich vu',
  PRIMARY KEY (`id`),
  KEY `idx_losu_changed_at` (`changed_at`),
  KEY `idx_losu_order_service_used_id` (`order_service_used_id`),
  CONSTRAINT `fk-log_order_service_used-order_service_used_id` FOREIGN KEY (`order_service_used_id`) REFERENCES `order_service_used` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Lịch sử thay đổi dịch vụ trong đơn hàng';
