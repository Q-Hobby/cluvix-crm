# `defer` vs `t.Cleanup` trong test Go

> Tách từ `skill/skills/testing-backend/SKILL.md` (2026-09-22, file gốc vượt 500 dòng).

### ⚠ `defer` chạy TRƯỚC `t.Cleanup` — đóng tài nguyên bằng `defer` làm cleanup hỏng IM LẶNG

`defer` chạy khi hàm test return, tức **trước** mọi `t.Cleanup`. Đóng pool/tắt server bằng `defer`
⇒ mọi lệnh dọn ở `t.Cleanup` bắn vào thứ đã đóng và **thất bại không báo gì**; test chạy sau nhận
dữ liệu bẩn rồi fail vì lý do sai (đã gặp: cleanup không khôi phục được 20 dòng, test sau đo 117
thay vì 137).

- Tài nguyên mà cleanup CẦN dùng (pool, connection) → đóng bằng `t.Cleanup`, đăng ký **trước** cái
  cần chạy sau: cleanup chạy **LIFO**, đăng ký trước = chạy sau cùng.
- Lệnh khôi phục dữ liệu trong cleanup phải kiểm `.Error` và `t.Errorf` khi hỏng — không kiểm thì
  đúng cái lưới dọn dẹp lại là chỗ hỏng âm thầm.

```go
wDB, _ := writer.DB()
t.Cleanup(func() { _ = wDB.Close() })          // đăng ký TRƯỚC ⇒ chạy SAU CÙNG
t.Cleanup(func() {
    if err := writer.Exec("UPDATE ... SET isLock = 1 WHERE id IN ?", ids).Error; err != nil {
        t.Errorf("KHÔI PHỤC THẤT BẠI — test sau sẽ sai: %v", err)
    }
})
```
