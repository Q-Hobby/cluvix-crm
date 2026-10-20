-- Remember-device 2FA: thay việc tin `fingerprint` (client tự gửi, đoán được) bằng token ngẫu nhiên
-- server cấp sau khi verify 2FA. Cột chỉ lưu SHA-256 hex của token; token thô nằm trong cookie
-- HttpOnly của trình duyệt. Row cũ có totp_trusted_until nhưng hash NULL ⇒ không còn được tin,
-- user nhập 2FA lại một lần. Idempotent: guard information_schema.
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_device' AND COLUMN_NAME='totp_trust_hash') = 0,
  'ALTER TABLE user_device ADD COLUMN totp_trust_hash CHAR(64) DEFAULT NULL COMMENT ''SHA-256 hex của token remember-device (cookie HttpOnly)'' AFTER totp_trusted_until',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
