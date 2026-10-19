-- Seed danh mục tối thiểu để CRM chạy được trên DB rỗng (story-26).
--
-- Nguyên tắc chọn: CHỈ seed dòng mà code đọc tới (theo id hoặc bắt buộc để luồng nghiệp vụ chạy),
-- cộng vài danh mục trung tính để màn hình không trống. Danh mục mang chất chuyên ngành (loại dịch
-- vụ, đơn vị tính trong `migrations_backup/seed/*.json` là nha khoa) KHÔNG seed — mỗi doanh nghiệp
-- tự khai trong màn Cấu hình.
--
-- Khoá seed (idempotent, chạy lại không nhân dòng):
--   * Bảng mà code tham chiếu dòng THEO ID (hằng trong `pkg/define/` hoặc literal trong SQL) ⇒ id
--     CHÍNH LÀ khoá seed, phải cố định: hr_department, user_option, payment_method,
--     appointment_is_accepted, appointment_type. Các bảng này đều có thêm khoá tự nhiên:
--       - có UNIQUE thật trên `name` (hr_department, user_option, payment_method,
--         appointment_is_accepted) ⇒ `INSERT IGNORE` bám cả PK lẫn UNIQUE(name);
--       - không có UNIQUE (appointment_type) ⇒ `WHERE NOT EXISTS` theo `name` + `INSERT IGNORE`
--         chặn trùng PK khi người dùng đã đổi tên dòng.
--   * Bảng không có dòng nào được code gọi theo id (source, customer_category) ⇒ KHÔNG ghi id,
--     `WHERE NOT EXISTS` theo `name`.
-- created_by/updated_by = NULL (seed hệ thống, không gắn người tạo).

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. hr_department — id theo `pkg/define/hr.go` (HrDepartment*). Telesale = 3 là phòng mà scope
--    khách hàng telesale (`crm/customer/scope`) và báo cáo telesale lọc theo; các id còn lại được
--    FE (`user.define.ts` DEPARTMENT_SELECT) và BE đọc theo hằng. Chỉ seed các id có hằng.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `hr_department` (`id`, `name`, `desc`, `status`, `SortOrder`, `created_at`, `updated_at`, `created_by`, `updated_by`)
VALUES
    (1,  'Phòng nhân sự',              NULL, 1, 1,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (2,  'Phòng lễ tân',               NULL, 1, 2,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (3,  'Phòng telesale',             NULL, 1, 3,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (4,  'Phòng marketing',            NULL, 1, 4,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (5,  'Phòng kế toán',              NULL, 1, 5,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (6,  'Phòng chăm sóc khách hàng',  NULL, 1, 6,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (9,  'Ban giám đốc',               NULL, 1, 9,  UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (11, 'Phòng chuyển đổi',           NULL, 1, 11, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (12, 'Nhân viên kinh doanh',       NULL, 1, 12, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL);

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. user_option — công tắc toàn hệ thống, code đọc THEO ID (`Where("id = ?", …)`):
--    BE: 1 (crm/payment helpers), 3 (define.UserOptionUpdateOrderPlan), 5 (define.UserOptionViewLeadOfTele),
--        6 (define.UserOptionDeleteNote), 12 (customer helpers), 14 (phoneutil ẩn SĐT),
--        18 (pkg/tenant IsCrossBranchEnabled), 20 (phoneutil export SĐT),
--        23 (define.UserOptionCrossBranchService).
--    FE (`config-general.define.ts` CONFIG_OPTION_CRM_SELECT, màn CRM còn dùng): 16, 17, 21, 22.
--    Mọi option mặc định TẮT (options = 0). Riêng id 5 = 0 nghĩa là BẬT giới hạn: nhân viên
--    telesale chỉ thấy khách mình phụ trách.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `user_option` (`id`, `name`, `desc`, `options`, `status`, `SortOrder`, `created_at`, `updated_at`, `created_by`, `updated_by`)
VALUES
    (1,  'Cập nhật thanh toán trong ngày', 'Nhân sự có quyền tạo thanh toán nhưng không có quyền cập nhật vẫn được sửa thanh toán do mình tạo trong ngày', 0, 1, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (3,  'Cập nhật đơn hàng trong ngày', 'Nhân sự có quyền tạo đơn hàng nhưng không có quyền cập nhật vẫn được sửa đơn hàng do mình tạo trong ngày', 0, 1, 3, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (5,  'Danh sách khách của telesale', 'Bật: nhân viên telesale thấy mọi khách hàng. Tắt: chỉ thấy khách mình phụ trách', 0, 1, 5, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (6,  'Xoá ghi chú trong ngày', 'Nhân sự có quyền tạo ghi chú nhưng không có quyền xoá vẫn được xoá ghi chú do mình tạo trong ngày', 0, 1, 6, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (12, 'Cho phép telesale cập nhật khách của người khác', 'Tắt: vẫn tìm kiếm được nhưng không sửa được thông tin khách hàng do người khác phụ trách', 0, 1, 12, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (14, 'Ẩn số điện thoại', 'Ẩn số điện thoại khách hàng, chỉ hiển thị 4 số cuối', 0, 1, 14, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (16, 'Cho phép chọn ngày trước đó', 'Cho phép chọn ngày trong quá khứ thay vì chỉ ngày hôm nay khi nhập liệu', 0, 1, 16, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (17, 'Cho phép chọn ngày hôm qua', 'Cho phép chọn ngày hôm qua khi tạo đơn hàng/thanh toán', 0, 1, 17, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (18, 'Hiển thị dữ liệu toàn chi nhánh', 'Cho phép các chi nhánh tìm kiếm và xem dữ liệu của nhau', 0, 1, 18, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (20, 'Hiển thị số điện thoại khi export', 'Bật: mọi người dùng thấy số điện thoại đầy đủ trong file export. Tắt: chỉ Ban giám đốc thấy', 0, 1, 20, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (21, 'Tính số lần sử dụng dịch vụ theo tổng gói', 'Số lần được sử dụng dịch vụ tính theo tổng số tiền đã đóng của cả gói', 0, 1, 21, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (22, 'Bỏ qua kiểm tra thanh toán khi sử dụng dịch vụ', 'Bật: khách có đơn hàng được sử dụng dịch vụ bất kể đã thanh toán hay chưa', 0, 1, 22, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (23, 'Cho phép dùng dịch vụ đã mua ở chi nhánh khác', 'Bật: dịch vụ mua ở một chi nhánh vẫn hiển thị và sử dụng được ở chi nhánh khác. Tắt: chỉ dùng tại chi nhánh đã mua', 0, 1, 23, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL);

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. payment_method — id 3 "Chuyển cọc" là hằng `define.PaymentMethodTransferDeposit` (chuyển tiền
--    cọc thành thanh toán, MoveDepositToPayment). Các id còn lại là phương thức phổ biến.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `payment_method` (`id`, `name`, `SortOrder`, `status`, `created_at`, `updated_at`, `created_by`, `updated_by`)
VALUES
    (1, 'Chuyển khoản', 2, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (2, 'Máy POS',      3, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (3, 'Chuyển cọc',   4, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (4, 'Tiền mặt',     1, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL);

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. appointment_is_accepted — id 1 = `define.IsAccepted`, id 2 = `define.IsNotAccepted`; báo cáo
--    CRM/dashboard lọc literal `is_accepted = 1`. `isAccepted = 1` đánh dấu trạng thái tính là
--    "đồng ý/chốt" (appointment service đọc `WHERE isAccepted = 1`) ⇒ chỉ dòng id 1.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `appointment_is_accepted` (`id`, `name`, `severity`, `desc`, `SortOrder`, `status`, `isAccepted`, `created_at`, `updated_at`, `created_by`, `updated_by`)
VALUES
    (1, 'Đồng ý',             'success', NULL, 1, 1, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (2, 'Không đồng ý',       'danger',  NULL, 2, 1, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (3, 'Hẹn làm sau',        'warn',    NULL, 3, 1, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (4, 'Khách đặt cọc',      'info',    NULL, 4, 1, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL),
    (5, 'Chọn dịch vụ khác',  'info',    NULL, 5, 1, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL);

-- ─────────────────────────────────────────────────────────────────────────────
-- 5. appointment_type — id 1 = lịch hẹn TƯ VẤN: `crm/appointment/handler/appointment.go` lọc literal
--    `a.appointment_type_id = 1`. Không có UNIQUE ⇒ `WHERE NOT EXISTS` theo `name`, `INSERT IGNORE`
--    đỡ trùng PK khi dòng id đó đã bị đổi tên.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `appointment_type` (`id`, `name`, `isTypeRevenue`, `status`, `SortOrder`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT v.id, v.name, 0, 1, v.id, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM (VALUES
    ROW(1, 'Tư vấn'),
    ROW(2, 'Sử dụng dịch vụ'),
    ROW(3, 'Hẹn lại'),
    ROW(4, 'Bảo hành')
) AS v (id, name)
WHERE NOT EXISTS (SELECT 1 FROM `appointment_type` x WHERE x.`name` = v.name);

-- ─────────────────────────────────────────────────────────────────────────────
-- 6. source (nguồn khách) + customer_category (nhóm khách) — danh mục trung tính cho form khách
--    hàng không trống. Code không gọi theo id ⇒ không ghi id, khoá seed = `name`.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO `source` (`name`, `status`, `SortOrder`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT v.name, 1, v.sort_order, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM (VALUES
    ROW('Khách vãng lai', 1),
    ROW('Người quen giới thiệu', 2),
    ROW('Facebook', 3),
    ROW('Google', 4),
    ROW('Website', 5)
) AS v (name, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM `source` x WHERE x.`name` = v.name);

INSERT INTO `customer_category` (`name`, `status`, `SortOrder`, `time_nhaclich`, `type_nhaclich`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT v.name, 1, v.sort_order, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM (VALUES
    ROW('Khách tiềm năng', 1),
    ROW('Khách thân thiết', 2),
    ROW('Không liên lạc được', 3)
) AS v (name, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM `customer_category` x WHERE x.`name` = v.name);
