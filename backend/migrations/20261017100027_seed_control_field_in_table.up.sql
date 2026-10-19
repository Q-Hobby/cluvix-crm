-- Seed 2 dòng `control_field_in_table` (story-30 P5). BE `GET/POST /config/control-field-in-table/:id/...`
-- tra `First(&item, id)` và FE gọi CỐ ĐỊNH id 1 (form khách hàng) / id 2 (form lead) ⇒ thiếu dòng thì
-- 404 và 2 trang `/config/customize/form-customer|form-lead` văng notfound. Giống seed ERP gốc:
-- `control`/`add_on` NULL ⇒ handler trả `defaultCustomerFields`. Idempotent: INSERT IGNORE bám PK `id`
-- + UNIQUE `name` (cả hai là khoá thật) — chạy lại không nhân bản, không đè cấu hình đã lưu.
INSERT IGNORE INTO `control_field_in_table` (`id`, `name`, `control`, `add_on`) VALUES
  (1, 'Bảng khách hàng (contact)', NULL, NULL),
  (2, 'Bảng khách hàng (Lead)', NULL, NULL);
