-- Seed dòng DUY NHẤT của `config_systems` (QA story-43 crm-standalone, 2026-10-17).
--
-- Vì sao: `20260928093146_create_config_systems` chỉ tạo bảng, KHÔNG seed dòng nào. Mọi chỗ ghi
-- (`pkg/dexie.BumpDexieTable`, `ConfigSystemsSetOption`) là `UPDATE config_systems SET content = …`
-- không có WHERE ⇒ bảng rỗng thì UPDATE 0 dòng, im lặng. Hệ quả: header `X-Dexie-Versions` luôn rỗng
-- ⇒ FE sau lần full sync đầu KHÔNG BAO GIỜ đồng bộ lại danh mục IndexedDB (chi nhánh, nhân viên,
-- dịch vụ, nguồn…) và không reload khi hồ sơ doanh nghiệp đổi.
--
-- Idempotent: chỉ chèn khi bảng còn trống (`WHERE NOT EXISTS`) — chạy lại không sinh dòng thứ hai,
-- không đè `content` đang có.
INSERT INTO `config_systems` (`content`)
SELECT JSON_OBJECT()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `config_systems`);
