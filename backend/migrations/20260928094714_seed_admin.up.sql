-- Seed khởi tạo: 1 chi nhánh mặc định + mã tự sinh (config_code) + tài khoản quản trị `admin`.
--
-- ⚠ Mật khẩu mặc định của `admin` được công bố trong README (story-29). Người cài đặt PHẢI đổi
-- mật khẩu ngay sau lần đăng nhập đầu tiên — `must_change_password = 1` ép đổi ở lần đăng nhập đầu.
-- Hash bcrypt (cost 12, khớp `hr/user/service` GenerateFromPassword(…, 12)) sinh MỚI cho repo này,
-- không dùng lại bất kỳ hash nào của hệ cũ.
--
-- Khoá seed — kiểu "chỉ khởi tạo khi bảng còn trống" cho các dòng có thể bị người vận hành đổi
-- khoá tự nhiên (mã chi nhánh, username): chạy lại migration sau khi đã đổi tên KHÔNG được tạo lại
-- chi nhánh thứ hai hay một tài khoản `admin` thứ hai mang mật khẩu mặc định công khai.
--   * branch      : `WHERE NOT EXISTS (SELECT 1 FROM branch)` + `INSERT IGNORE` (UNIQUE code/name).
--   * user        : `WHERE NOT EXISTS (SELECT 1 FROM user)`   + `INSERT IGNORE` (UNIQUE username).
--   * user_profile: PK user_id ⇒ `INSERT IGNORE`.
--   * rbac role   : PK name ⇒ `INSERT IGNORE`.
--   * assignment  : chỉ gán khi CHƯA ai giữ role `admin` (người vận hành đã thu hồi/chuyển role thì
--                   chạy lại không cấp lại) + `INSERT IGNORE` (PK user_id,item_name).
--   * config_code : UNIQUE (branch_id, parent_table) ⇒ `INSERT IGNORE`; riêng `service` là 1 dòng
--                   dùng chung toàn hệ thống ⇒ `WHERE NOT EXISTS parent_table = 'service'`.
-- created_by/updated_by = NULL (seed hệ thống).

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. Chi nhánh mặc định. Mọi scope dữ liệu (tenant.ScopedDBNullBranch, user.branchId,
--    user_profile.branchActive_id) và codegen đều cần ít nhất 1 chi nhánh.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `branch` (`name`, `code`, `status`, `SortOrder`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT 'Chi nhánh chính', 'CN01', 1, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `branch`);

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. config_code — mẫu mã tự sinh, khớp `defaultConfigCodeTemplates` + `SeedConfigCodeForBranch`
--    (`config/branch/handler/handler.go`), vì chi nhánh tạo bằng SQL không đi qua handler. Thiếu
--    dòng thì `pkg/codegen.Generate` trả mã rỗng ⇒ bảng có UNIQUE(code) (appointment, payment,
--    order_plan, service…) vỡ 1062 từ bản ghi thứ hai.
--    Seed cho MỌI chi nhánh chưa có (không neo 1 chi nhánh): mặc định cột include_branch_code=1,
--    monthly_reset=1, serial_length=2 như handler.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `config_code` (`branch_id`, `name`, `parent_table`, `infix_code`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT b.id, t.name, t.parent_table, t.infix_code, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM `branch` b
CROSS JOIN (VALUES
    ROW('Khách hàng',      'customer',           'KH'),
    ROW('Đơn hàng',        'order_plan',         'DH'),
    ROW('Thanh toán',      'payment',            'TT'),
    ROW('Lịch hẹn',        'appointment',        'LH'),
    ROW('Nhắc lịch',       'reminder',           'RE'),
    ROW('Dịch vụ sử dụng', 'order_service_used', 'SV')
) AS t (name, parent_table, infix_code);

-- Mã dịch vụ: 1 dòng dùng chung toàn hệ thống (không ghép mã chi nhánh, không reset theo tháng,
-- serial 3 chữ số) — đặt trên chi nhánh id nhỏ nhất như handler đặt trên chi nhánh tạo đầu tiên.
INSERT IGNORE INTO `config_code` (`branch_id`, `name`, `parent_table`, `infix_code`, `include_branch_code`, `monthly_reset`, `serial_length`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT b.id, 'Dịch vụ', 'service', 'DV', 0, 0, 3, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM `branch` b
WHERE b.id = (SELECT MIN(id) FROM `branch`)
  AND NOT EXISTS (SELECT 1 FROM `config_code` WHERE `parent_table` = 'service');

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. Tài khoản quản trị `admin` (status 10 = define.UserStatusActive), gán chi nhánh mặc định.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `user` (`username`, `password_hash`, `must_change_password`, `branchId`, `status`, `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT 'admin', '$2a$12$iNdvIu1kH0qt7jlsgZtixeTBLpRyykRHwFxVS5el3hlIUxC7fRrQK', 1,
       JSON_ARRAY((SELECT MIN(id) FROM `branch`)), 10, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `user`);

-- Hồ sơ trung tính (không email/SĐT — hai cột UNIQUE, NULL không đụng nhau).
INSERT IGNORE INTO `user_profile` (`user_id`, `fullname`, `locale`, `departmentList`, `branchOffice_id`, `branchActive_id`, `updated_at`)
SELECT u.id, 'Quản trị viên', 'vi-VN', JSON_ARRAY(),
       (SELECT MIN(id) FROM `branch`), (SELECT MIN(id) FROM `branch`), UNIX_TIMESTAMP()
FROM `user` u
WHERE u.username = 'admin';

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. RBAC. Role `admin` thuộc `define.SuperRoles` ⇒ có MỌI quyền qua kiểm tra super-role, KHÔNG
--    cần dòng rbac_auth_item_child. Catalog quyền (type = 2) KHÔNG seed ở đây: app tự upsert từ
--    `auth/rbac/seed/permissions.gen.json` mỗi lần khởi động (`bootstrap.seedPermissions` →
--    `RbacService.SeedPermissions`) — seed trùng bằng SQL sẽ thành nguồn thứ hai trôi lệch.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT IGNORE INTO `rbac_auth_item` (`name`, `type`, `description`, `rule_name`, `created_at`, `updated_at`)
VALUES ('admin', 1, 'Quản trị hệ thống', NULL, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT IGNORE INTO `rbac_auth_assignment` (`item_name`, `user_id`, `created_at`)
SELECT 'admin', u.id, UNIX_TIMESTAMP()
FROM `user` u
WHERE u.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM `rbac_auth_assignment` WHERE `item_name` = 'admin');
