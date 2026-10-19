# Lưới chặn test chạm DB thật — vì sao fail-closed bằng quy ước tên

> Bổ trợ cho mục ⛔ trong `../SKILL.md`. Đọc khi sửa `internal/testhelpers/suite.go`,
> khi bị guard chặn, hoặc khi định nới quy ước đặt tên DB test.

## Luật

Tên DB trong `TEST_MYSQL_DSN` **phải kết thúc** bằng `_test` hoặc `_tmp`. Hậu tố, không phải
tiền tố: `test_cluvix_x` bị từ chối, `cluvix_x_test` mới đạt. `cluvix_test_backup` cũng bị từ
chối — `_test` nằm giữa không tính.

Cưỡng chế ở `assertDisposableDB` (`internal/testhelpers/suite.go`), chạy **trước** `gorm.Open`.

⛔ **Quy ước tên bảo vệ lúc CHẠY, KHÔNG bảo vệ lúc XOÁ.** Dọn đĩa chỉ được `DROP` các DB mang
tiền tố mà chính phiên/agent này đặt ra và đã ghi lại (vd `cluvix_<epic>_*`) — cấm quét theo mẫu
chung `%_tmp` / `%_test`. "Dùng một lần" là với NGƯỜI TẠO, không phải với người dọn: phiên khác có
thể đang giữa một phép đo dài trên đúng DB đó (2026-09-07: xoá nhầm `cluvix_scale_tmp` ~487MB của
phiên `/check-scale`, không khôi phục được). Prompt cho executor phải nêu tiền tố bắt buộc và yêu
cầu tự drop khi xong.

## Vì sao không chỉ so với `DB_NAME` (sửa 2026-09-02)

Guard cũ:

```go
if realDB := os.Getenv("DB_NAME"); realDB != "" && dbNameFromDSN(dsn) == realDB {
```

Nhưng **không một `*_test.go` nào trong repo nạp `.env`** — đo 2026-09-02: 0 file dùng
`godotenv` (đối chứng: 461 file có `testing`, nên lệnh grep có chạy thật). Trong process test
`DB_NAME` luôn rỗng ⇒ `realDB != ""` short-circuit ⇒ **guard chưa từng bắn một lần nào**, trong
khi comment ngay trên nó ghi "Fatal ngay trước khi connect".

Bài học lớp này: một lưới phụ thuộc vào biến môi trường mà không ai set là lưới **trang trí** —
và nó nguy hiểm hơn không có lưới, vì người đọc code tin là đã được bảo vệ. Xem
`feedback_dont_trust_only_emit_point_comment`.

## Hai tầng hiện tại

| Tầng | Bắt gì | Còn đúng khi |
|---|---|---|
| 1. Quy ước tên `_test`/`_tmp` | mọi DB không phải dùng-một-lần | không đọc được `.env`, không có `DB_NAME` |
| 2. So với DB thật (env → `.env`) | DB dev lỡ đặt tên hợp quy ước | đọc được tên DB thật |

Tầng 1 là cái làm nó *fail-closed*: không cần biết DB thật tên gì vẫn chặn đúng.

## Khoá bằng test

`internal/testhelpers/suite_guard_test.go` — `TestAssertDisposableDB` (6 case, có ca
`cluvix_test_backup` để bắt ai đó đổi `HasSuffix` thành `Contains`), `TestRealDBName_EnvWins`,
`TestBackendRootHasEnvOrMigrations`. Đã mutation-test: đổi `HasSuffix` → `Contains` làm test đỏ
đúng ca đó.

## Lớp ngoài cùng: hook Bash

`skill/hooks/guard-bash.sh` §3 chặn `go test` / migrate / `mysql` GHI-HUỶ nhắm `DB_NAME` ngay ở
tầng Bash. Hai lưới **không thừa nhau**: hook phủ cả `mysql`/`migrate` nhưng chỉ khi lệnh đi qua
agent; `assertDisposableDB` phủ cả khi test chạy từ GoLand/CI.
