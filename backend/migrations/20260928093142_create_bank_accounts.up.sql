-- Tạo bảng `bank_accounts` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `bank_accounts` (
  `id` int NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `bank_code` varchar(20) NOT NULL COMMENT 'Mã ngân hàng theo chuẩn VietQR/Napas (VCB, TCB, MB...)',
  `account_no` varchar(50) NOT NULL COMMENT 'Số tài khoản ngân hàng',
  `account_name` varchar(100) NOT NULL COMMENT 'Tên chủ tài khoản',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Trạng thái: 1=active, 0=tắt (không random vào)',
  `created_by` int DEFAULT NULL COMMENT 'Người tạo',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật',
  `sort_order` int NOT NULL DEFAULT '0' COMMENT 'Thứ tự hiển thị (BE tự set = id sau create)',
  `created_at` bigint NOT NULL DEFAULT '0' COMMENT 'Thời gian tạo (Unix timestamp)',
  `updated_at` bigint NOT NULL DEFAULT '0' COMMENT 'Thời gian cập nhật (Unix timestamp)',
  PRIMARY KEY (`id`),
  KEY `fk-bank_accounts-created_by` (`created_by`),
  KEY `fk-bank_accounts-updated_by` (`updated_by`),
  KEY `idx-bank_accounts-status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Danh sách tài khoản ngân hàng để sinh QR chuyển khoản';
