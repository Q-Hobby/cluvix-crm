-- Tạo bảng `attachment` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `attachment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `ref_type` varchar(50) NOT NULL COMMENT 'Loại đối tượng: treatment_course, appointment, medical_record',
  `ref_id` int NOT NULL COMMENT 'ID đối tượng',
  `customer_id` int NOT NULL COMMENT 'Khách hàng sở hữu file',
  `branch_id` int DEFAULT NULL,
  `file_name` varchar(255) NOT NULL COMMENT 'Tên file gốc',
  `file_url` varchar(500) NOT NULL COMMENT 'URL trên cloud (S3)',
  `file_type` varchar(100) DEFAULT NULL,
  `file_size` int DEFAULT NULL COMMENT 'Kích thước file (bytes)',
  `note` varchar(500) DEFAULT NULL COMMENT 'Ghi chú',
  `sort_order` int DEFAULT '0' COMMENT 'Thứ tự hiển thị',
  `created_at` bigint DEFAULT NULL,
  `created_by` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx-attachment-ref` (`ref_type`,`ref_id`),
  KEY `idx_attachment_customer` (`customer_id`),
  KEY `fk-attachment-branch_id` (`branch_id`),
  CONSTRAINT `fk-attachment-customer_id` FOREIGN KEY (`customer_id`) REFERENCES `customer` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk-attachment-branch_id` FOREIGN KEY (`branch_id`) REFERENCES `branch` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
