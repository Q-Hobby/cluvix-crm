# Màn danh sách lịch hẹn — hợp đồng phải giữ khi sửa

Nền tảng đo đạc, 4 biến thể tìm kiếm đã loại, quy trình bench → `skill/skills/debugging-backend-queries/reference/perf-list-endpoints.md`.
Bẫy correctness của chỉ mục ngram → `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md` §FULLTEXT `ngram`.
File này chỉ ghi phần riêng của `crm/appointment` — thứ phải cập nhật khi sửa list.

## Code map

| File | Mục đích |
|---|---|
| `crm/appointment/handler/list_search.go` | `needsCustomerJoin`, `applyKeywordFast`, `customerFilterColumns`, `keywordIDCap`, `ngramMinLen` |
| `crm/appointment/handler/appointment.go` | `BuildAppointmentQuery` gọi 2 hàm trên; `appointmentListColumns` / `appointmentDetailColumns` |
| `migrations/20260906122048_rebuild_ngram_omi_and_customer.up.sql` | 2 chỉ mục ngram (stopword TẮT) |

## LEFT JOIN customer có điều kiện

- `needsCustomerJoin(c)` = true khi `_export=1` HOẶC có filter trong `customerFilterColumns`: `_status`, `_gender`, `_page_facebook_id`, `customerType`, `category`, `_groupLocal`, `_groupCountry`.
- **Thêm filter mới đọc `customer.*` trong `BuildAppointmentQuery` → PHẢI thêm khoá vào `customerFilterColumns`** — thiếu là `Unknown column 'customer.x'`.
- `_export=1` ép join tại đây (một điểm quyết định duy nhất), KHÔNG để `export.ExportAppointment` tự join.

## `applyKeywordFast` — hợp đồng gọi

- Trả `(q, true)`: đã gắn điều kiện, caller KHÔNG join. Trả `(q, false)`: caller PHẢI tự `LEFT JOIN customer` + `query.ApplyKeywordFilter`.
- Cột ngram: `MATCH(fullname, code, nickname, phone, other_phone, old_code)` trên `customer` · `MATCH(code)` trên `appointment`.
- **Nháy kép bắt buộc** trong `AGAINST('"kw"' IN BOOLEAN MODE)`; bỏ nháy kép có sẵn trong từ khoá trước khi bọc. Thiếu nháy kép → ngram tách bigram rời rồi OR → THỪA kết quả.
- Từ khoá toàn số: thêm `CAST(id AS CHAR) LIKE '%kw%'` trên `customer` (`customer.id` nằm trong `appointmentKeySearch`).
- 2 truy vấn tra id KHÔNG scope tenant — CỐ Ý, giữ tập kết quả bằng bản cũ; lịch hẹn vẫn scope đầy đủ ở query chính.

## Cột list vs cột detail

- `appointmentListColumns` giữ hẹp; `appointmentDetailColumns` = list + nhóm tiếp nhận/sinh hiệu EMR (`emr_department_id`, `emr_room_id`, `emr_queue_number`, `emr_medical_record_id`, weight/height/bmi/blood_pressure/pulse/temperature/respiratory_rate/spo2, chief_complaint/symptoms/medical_history).
- Thiếu nhóm EMR ở detail → khối "Thông tin y tế" FE rỗng câm lặng, không lỗi.
