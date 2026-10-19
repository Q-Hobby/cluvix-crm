-- Quét cấu hình Meta TRÙNG ĐỊNH DANH trong một DB khách.
-- omnichat-meta-webhook-hardening story-04 — CHẠY TRƯỚC migration
-- `*_uniq_meta_page_config_identity.up.sql`.
--
-- Cách chạy (READ-ONLY, không sửa gì):
--   mysql -h <host> -u <user> -p <ten_db> < backend/scripts/scan_meta_duplicate_identity.sql
--
-- ĐỌC KẾT QUẢ:
--   ⚠ ĐỌC DÒNG CUỐI TRƯỚC: script kết thúc bằng một dòng `SCAN HOAN TAT — ...`. KHÔNG thấy dòng đó
--     nghĩa là script CHẾT GIỮA CHỪNG (sai schema, thiếu quyền, gõ nhầm tên DB) — "không có dòng nào"
--     lúc đó KHÔNG phải là "sạch". Đã đo thật khi viết story: thiếu một cột làm script dừng ở
--     `ERROR 1054` và output rỗng y hệt ca sạch.
--   • Thấy dòng kết thúc VÀ hai truy vấn không trả dòng nào → DB này sạch, apply migration được.
--   • Có bất kỳ dòng nào                  → ĐANG TRÙNG THẬT. DỪNG, đừng apply: migration sẽ fail
--     (Error 1062) và chặn cả đợt deploy. Mỗi dòng liệt kê các account đang giành nhau một định danh;
--     người vận hành phải quyết giữ cấu hình nào rồi gỡ kết nối phần còn lại qua giao diện.
--     ⛔ KHÔNG tự DELETE row ở đây: `meta_page_config` giữ page access token, xoá nhầm là mất kết nối
--     của một phòng khám đang chạy.
--
-- Vì sao trùng là hỏng: webhook Meta chỉ mang `recipient.id`; hai account cùng định danh thì không có
-- dữ kiện nào để biết tin thuộc phòng khám nào.

SELECT
  'page_id'                                   AS loai_dinh_danh,
  mpc.page_id                                 AS dinh_danh,
  COUNT(*)                                    AS so_cau_hinh,
  GROUP_CONCAT(mpc.account_id ORDER BY mpc.account_id)  AS account_ids,
  GROUP_CONCAT(acc.company_id ORDER BY mpc.account_id)  AS company_ids,
  GROUP_CONCAT(COALESCE(mpc.page_name, '?') ORDER BY mpc.account_id SEPARATOR ' | ') AS ten_page
FROM meta_page_config mpc
JOIN omni_channel_account acc ON acc.id = mpc.account_id
GROUP BY mpc.page_id
HAVING COUNT(*) > 1;

SELECT
  'ig_business_account_id'                    AS loai_dinh_danh,
  mpc.ig_business_account_id                  AS dinh_danh,
  COUNT(*)                                    AS so_cau_hinh,
  GROUP_CONCAT(mpc.account_id ORDER BY mpc.account_id)  AS account_ids,
  GROUP_CONCAT(acc.company_id ORDER BY mpc.account_id)  AS company_ids,
  GROUP_CONCAT(COALESCE(mpc.ig_username, '?') ORDER BY mpc.account_id SEPARATOR ' | ') AS ig_username
FROM meta_page_config mpc
JOIN omni_channel_account acc ON acc.id = mpc.account_id
-- NULL/rỗng = page chưa liên kết IG: hợp lệ và phổ biến, KHÔNG phải trùng.
WHERE mpc.ig_business_account_id IS NOT NULL AND mpc.ig_business_account_id <> ''
GROUP BY mpc.ig_business_account_id
HAVING COUNT(*) > 1;

-- Số cấu hình đang để `ig_business_account_id = ''` (chuỗi rỗng). KHÔNG phải lỗi: migration sẽ chuẩn
-- hoá chúng về NULL trước khi thêm UNIQUE (MySQL cho nhiều NULL nhưng KHÔNG cho nhiều ''). Dòng này
-- chỉ để người chạy biết migration sắp UPDATE bao nhiêu row.
SELECT COUNT(*) AS so_row_ig_rong_se_chuan_hoa_ve_null
FROM meta_page_config WHERE ig_business_account_id = '';

-- Mốc kết thúc — KHÔNG thấy dòng này = script chưa chạy hết, đừng kết luận "sạch".
SELECT CONCAT('SCAN HOAN TAT — db=', DATABASE(), ', tong_cau_hinh=', (SELECT COUNT(*) FROM meta_page_config)) AS trang_thai;
