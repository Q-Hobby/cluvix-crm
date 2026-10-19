# Storage (local / S3 / R2)

## Mục lục

- Thiết kế (2026-10-19)
- Cấu hình
- DB lưu KEY — đọc ghép URL, ghi bóc về key
- Kho private — file đính kèm khách hàng
- Bảng API
- Gotcha

---

> **Update/xoá ảnh (form field ảnh trong resource CRUD)?** → đọc `s3-imaging.md` mục "HandleFormImage KHÔNG tự xoá ảnh cũ" trước.

## Thiết kế (2026-10-19)

`pkg/storage.Store` (interface: `Save/Open/Delete/URL/Public`), driver chọn bằng `STORAGE_DRIVER`
trong `storage.New(cfg)` (`factory.go`) — trả **2 kho**:

| Kho | Dùng cho | `URL(key)` |
|---|---|---|
| public (`deps.Store`) | avatar KH/NV, logo/icon thương hiệu, ảnh chi nhánh/dịch vụ/nhóm DV, ảnh TinyMCE | CDN/static |
| private (`deps.PrivateStore`) | `crm/attachment` (đính kèm khách) | luôn `""` |

Sai/thiếu cấu hình ⇒ `bootstrap` trả lỗi, server **dừng khởi động** — KHÔNG rơi về local, KHÔNG kho nil.
`S3Client` cũ (ListObjects/CreateFolder/Presigned/SaveLocal…) đã gỡ: không còn caller.

## Cấu hình

| Driver | Public | Private |
|---|---|---|
| `local` (mặc định; trống + không `S3_KEY`) | `STORAGE_PATH`, router mount static tại `STORAGE_PUBLIC_PREFIX` (`internal/router/uploads.go`, không liệt kê thư mục) | `STORAGE_PRIVATE_PATH`, KHÔNG mount; trùng/lồng thư mục public ⇒ lỗi boot |
| `s3` (hoặc trống + có `S3_KEY` — suy ra, log cảnh báo) | `S3_BUCKET`, folder `S3_FOLDER`, ACL `S3_PUBLIC_ACL` (trống=public-read, `none`=không gửi) | `S3_PRIVATE_BUCKET` (trống=`S3_BUCKET`), folder `<S3_FOLDER>/private`, ACL private |
| `r2` | như s3, region `auto`, KHÔNG gửi ACL, checksum WhenRequired, `S3_CDN` bắt buộc | `S3_PRIVATE_BUCKET` bắt buộc + KHÁC `S3_BUCKET` (public R2 là cấp bucket) |

Local `STORAGE_PUBLIC_BASE` rỗng ⇒ URL tương đối `/uploads/…` (FE khác origin ⇒ ảnh 404, phải khai base
hoặc proxy `/uploads`).

## DB lưu KEY — đọc ghép URL, ghi bóc về key

- Cột ảnh lưu **KEY** (vd `customer/KH001/avatar/img-….webp`), KHÔNG domain, KHÔNG folder S3.
- **Đọc** (mọi response trả ảnh): `storage.PublicURL(store, v)` / `storage.ResolveURL(v)` (kho public mặc
  định, `SetDefaultPublic` ở bootstrap) / `ResolveFields|ResolveRows|ResolveNested`. URL tuyệt đối cũ
  (`http(s)://`) trả **nguyên** — tương thích ngược, không migration dữ liệu.
- Object lồng qua `handlerutil.AttachRelation*` tự ghép theo `storage.TableImageColumns` (customer.avatar,
  user_profile.avatar, branch/service/service_type.image, business_profile.logo_image/icon_image).
  **Thêm cột ảnh mới ⇒ thêm vào map này** + ghép ở mọi Select tường minh đọc cột đó.
- **Ghi** giá trị ảnh từ payload: `storage.ToKey(h.Store, v)` — FE gửi lại nguyên URL GET đã trả; không bóc
  thì DB lưu URL và `DeleteOldIfChanged` thấy "khác" ⇒ XOÁ NHẦM ảnh đang dùng.
- Struct entity: trả bản sao `withURL(item)` (đừng gán URL vào biến rồi `Save`).

## Kho private — file đính kèm khách hàng

- Upload `imaging.SaveUploadSmart(ctx, h.PrivateStore, …)`, key `crm/attachments/<customerID>/…`, đuôi theo
  MIME đã sniff; cột `attachment.file_url` lưu KEY.
- Tải: `GET /api/crm/attachment/:id/download` (perm `viewCustomer` + `customerVisible` = phạm vi
  GetCustomer gồm telesale) ⇒ stream, `Content-Disposition` inline (ảnh/PDF) / attachment, `nosniff`,
  `no-store`. Bản ghi cũ `file_url` là URL tuyệt đối ⇒ 302 tới URL đó.
- List/upload trả `download_path` (tương đối gốc API), KHÔNG trả `file_url`.
- Xoá: xoá bản ghi trong tx → xoá file → commit; xoá file lỗi ⇒ rollback + 500 (không nuốt lỗi).

## Bảng API

| Hàm | Ghi chú |
|---|---|
| `storage.New(cfg) (pub, priv, err)` | duy nhất chọn driver |
| `Store.Save(ctx, r, key, contentType)` | contentType = MIME đã sniff |
| `Store.Open(ctx, key)` | `ErrNotFound` khi thiếu; caller `Close` |
| `Store.Delete(ctx, key)` | thiếu file không phải lỗi |
| `storage.SafeKey(key)` | chặn rỗng/tuyệt đối/`..`/`\` — mọi driver tự gọi |
| `storage.SaveUpload(ctx, s, file, base, allowed)` | lưu nguyên bản (whitelist magic bytes, đuôi theo MIME) |
| `storage.PublicURL` / `ToKey` / `ResolveURL` | xem trên |

## Gotcha

- S3 private dùng CHUNG bucket mà bucket có policy public-read toàn bucket ⇒ file private vẫn đọc được
  (policy thắng ACL) — factory log cảnh báo; giới hạn policy hoặc đặt `S3_PRIVATE_BUCKET`.
- Nil pointer AWS SDK `*string` (`ContentType`, `ContentLength`) — dùng `aws.ToString`/check nil.
- Test driver S3/R2 bằng `httptest` giả lập endpoint (`pkg/storage/s3_test.go`), KHÔNG gọi cloud thật.
