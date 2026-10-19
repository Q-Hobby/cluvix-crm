# Runtime Diagnostics

## Mục lục

- Bảng `app_event_log` + `pkg/applog`
- Workflow — kiểm chứng logic trên hệ thống đang chạy
- Rule khi thêm log mới
- Query chậm

---


Dùng khi cần biết "vì sao nhánh này không chạy đúng trên máy thật", thay cho SSH grep `backend/tmp/run.log`.

## Bảng `app_event_log` + `pkg/applog`

- Ghi: `applog.Info/Warn/Error(applog.Entry{...})` — async, best-effort, KHÔNG BAO GIỜ chặn hay làm hỏng nghiệp vụ.
- Chưa `applog.Init(db)` (test/CLI) → mọi `Write` là no-op. Không cần guard ở call site.
- Trường: `level`, `source` (namespace chấm), `event` (mã ngắn ỔN ĐỊNH), `message`, `company_id`/`branch_id`/`user_id`, `ref_type`+`ref_id` (đa hình, không FK), `request_id`, `error_code`, `data` JSON tuỳ ý.
- `error_code` để RIÊNG mã provider (vd `-216`) → `GROUP BY` đếm được; KHÔNG nhét vào `message`.
- `event` đặt rồi thì KHÔNG đổi chữ — mọi câu đếm/cảnh báo bám vào nó.
- CẤM đưa token/secret/mật khẩu và dữ liệu cá nhân (tên khách, SĐT) vào `data` — bảng nằm trong mọi bản dump DB, giữ 30 ngày.
- Hàng đợi đầy → DROP; kiểm độ đầy đủ bằng `applog.Dropped()`.
- Dọn bảng: job `system.app_event_log_prune` (cron) đã bỏ 2026-10-17 cùng scheduler ⇒ hiện KHÔNG còn job dọn `app_event_log`; bảng chỉ lớn lên — tự dọn tay hoặc thêm tác vụ `helpers.RunBackground` nếu cần.

```go
applog.Warn(applog.Entry{
    Source: "omnichat.zalo.profile", Event: "fetch_failed",
    Message:   "Zalo không trả về tên khách",
    CompanyID: applog.UintPtr(companyID),
    RefType:   "omi_conversation", RefID: applog.Uint64Ptr(convID),
    ErrorCode: zaloErrCode(err),
    Data:      map[string]any{"account_id": accountID, "ly_do": errText(err)},
})
```

## Workflow — kiểm chứng logic trên hệ thống đang chạy

1. Xác định các NHÁNH nghi ngờ (mỗi nhánh 1 `event` riêng), gồm cả nhánh **thành công** — không có ca thành công thì không phân biệt được "tính năng chết" với "tính năng chạy, riêng ca này provider từ chối".
2. Cắm `applog` vào từng nhánh, kèm `ref_type`+`ref_id` để JOIN ngược, và `error_code` khi có mã từ provider.
3. Chạy nghiệp vụ thật (hoặc chờ traffic thật).
4. Đối chiếu bằng SQL, KHÔNG đọc file log:

```sql
SELECT error_code, COUNT(*) FROM app_event_log
WHERE source='<source>' AND event='<event>' AND created_at > UNIX_TIMESTAMP()-7*86400
GROUP BY error_code ORDER BY 2 DESC;
```

5. Kết luận theo phân bố mã lỗi. Không có dòng nào = nhánh KHÔNG chạy (khác hẳn với chạy mà lỗi) — kiểm điều kiện spawn trước khi đổ cho provider.

## Rule khi thêm log mới

- Log sự kiện ĐÁNG TRUY VẤN LẠI (thưa, có ngữ cảnh) → `applog`. Dòng chảy vận hành dày đặc → giữ ở zap.
- Ghi CẢ hai: zap cho vận hành realtime, applog cho truy vấn lại. Không bỏ zap.
- Không gọi `applog` bên trong đường ghi lỗi của chính `applog` (vòng lặp vô hạn).

## Query chậm

`pkg/logger/gorm.go` log `[SLOW SQL <thời gian>] @ <path> rows=N | <câu SQL>` ra **stderr** khi query ≥200ms (`SlowThreshold`) → đọc ở `backend/tmp/run.log`.

stderr chứ không `L.Error`: `L.Error` bắn Telegram alert, báo cáo nặng sẽ spam kênh.

`Trace` chỉ log lỗi khi `err != nil` — không có nhánh này thì query 10s hoàn toàn vô hình.
