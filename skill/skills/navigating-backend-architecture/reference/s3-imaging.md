# Imaging (ảnh → kho public)

> Từ 2026-10-19 mọi hàm nhận `storage.Store` (không còn `*storage.S3Client`) và trả/ghi **KEY**
> (đường dẫn tương đối), KHÔNG phải URL. Trả FE phải ghép URL: `storage.PublicURL`/`ResolveURL` —
> xem `s3-storage.md` §DB lưu KEY. Tên file trong kho KHÔNG chứa tên client gửi (`img-<rand>-<unix>.<ext>`),
> đuôi/Content-Type theo MIME đã sniff.

## Mục lục

- Flow
- API — 2 pattern chính
  - Pattern A: form field → ghi vào model field
  - Pattern B: form field → lấy URL string
  - Pattern D: luồng file HỖN HỢP (ảnh lẫn PDF/video/office)
  - Pattern C: dọn ảnh cũ sau khi Save/Delete (xem checklist ⚠ bên dưới)
  - Khác
- 3 Resize Modes
- Presets
- Safeguards
  - Semaphore: kiểm `ctx.Err()` TRƯỚC `select`
- Lưu ý
- ⚠ HandleFormImage KHÔNG tự xoá ảnh cũ — bug pattern đã gặp (branch, company logo/icon 2026-07-16)
- Temp file trên server — phải xoá sau khi upload S3 xong

---


## Flow
```
Upload file
  ├─ Extension không hợp lệ (jpg/png/gif/webp)? → Error
  ├─ Size > 10MB? → Error
  ├─ Size ≤ 200KB / .webp / .gif? → Passthrough (upload gốc)
  └─ .jpg/.png lớn → semaphore(4) → EXIF fix → resize → sharpen → WebP encode (binary search quality 60-92, target ~160KB) → pick bản nhỏ hơn → Store.Save
```

## API — 2 pattern chính

### Pattern A: form field → ghi vào model field
```go
imaging.HandleFormImage(c, h.Store, "fileImage", "folder/", &item.Image, nil) // ghi KEY vào item.Image
// Không có file → return nil (không lỗi) — ĐÚNG cho form CUD (ảnh optional)
```

Endpoint **chuyên-đổi-ảnh** (chỉ ghi mỗi cột ảnh) phải tự guard TRƯỚC khi gọi — không guard thì thiếu part file = `200` + bản ghi cũ, client không phân biệt được với thành công:
```go
if _, err := c.FormFile("image"); err != nil { response.Validate(c, i18n.T(c, "image_required")); return }
if h.Store == nil { response.ServerError(c, fmt.Errorf("storage chưa được cấu hình")); return } // lỗi hạ tầng, KHÔNG Validate 422
```

### Pattern B: form field → lấy KEY
```go
if file, _ := c.FormFile("fileImage"); file != nil && h.Store != nil {
    key, err := imaging.ProcessAndUpload(c.Request.Context(), h.Store, file, "folder/", nil)
}
```

### Pattern D: luồng file HỖN HỢP (ảnh lẫn PDF/video/office)
```go
res, err := imaging.SaveUploadSmart(c.Request.Context(), h.PrivateStore, file, "folder/", opts)
// res.Key / res.MIME / res.Size / res.Filename — phản ánh file SAU xử lý trong kho
```
Phân nhánh theo magic bytes: ảnh → pipeline, còn lại → lưu nguyên bản. Metadata ghi DB phải lấy từ
`res`, KHÔNG từ `header.Header.Get("Content-Type")`/`header.Size` (ảnh đã thành WebP).

### Pattern C: dọn ảnh cũ sau khi Save/Delete (xem checklist ⚠ bên dưới)
```go
imaging.DeleteOldIfChanged(h.Store, oldImage, item.Image) // best-effort, chuẩn hoá cả 2 về KEY (ToKey) trước khi so
```

### Khác
- `ProcessFile(ctx, file, opts)` → chỉ process, tự lưu sau bằng `UploadProcessed(ctx, store, result, base)` (trả KEY)
- `ProcessReader(ctx, reader, filename, opts)` → process từ io.ReadSeeker; KHÔNG có nhánh passthrough, luôn decode→resize→WebP
- `EncodeWebPTargetSizeCtx(ctx, img, targetKB, qMin, qMax)` → chỉ encode, không resize
- `GuardImageReader(r)` → guard ngưỡng pixel cho ảnh KHÔNG đến từ multipart (vd tải từ URL)

`ctx` là tham số ĐẦU của 3 hàm public. `HandleFormImage`/`HandleFormImageRaw` giữ nguyên chữ ký, tự truyền `c.Request.Context()`.

## 3 Resize Modes

| Mode | Mô tả | Use case |
|------|--------|----------|
| `ModeFit` (default) | Fit box, giữ tỷ lệ, không upscale | Ảnh thường |
| `ModeFill` | Crop center đúng kích thước | Avatar, coupon, icon |
| `ModeRaw` | Upload gốc, chỉ validate | Template, file giữ nguyên |

## Presets

| Preset | Mode | Size | TargetKB | Use case |
|--------|------|------|----------|----------|
| `nil` (default) | Fit | 1280×1280 | 160 | Ảnh thường |
| `OptAvatar()` | Fill | 500×500 | 100 | **Avatar người (nhân viên + khách hàng)** — mọi khung hiển thị là tròn/vuông, KHÔNG dùng `nil` |
| `OptFill500()` | Fill | 500×500 | 100 | Coupon image |
| `OptFill200()` | Fill | 200×200 | 50 | Thumbnail |
| `OptRaw()` | Raw | — | — | Giữ nguyên gốc |
| `OptHighQuality()` | Fit | 1920×1080 | 300 | Banner (hiện 0 call site) |
| `OptMedical()` | Fit | 2048×2048 | 700 | Ảnh y tế — Q88–95, **`Sharpen: false`** |

KHÔNG bật `Sharpen` cho ảnh chẩn đoán — sharpen tạo cạnh giả, dễ đọc nhầm là ranh giới tổn thương.

## Safeguards
- Chỉ chấp nhận jpg/png/gif/webp, max 10MB. `.heic/.heif` từ chối tường minh (`ErrHEICNotSupported`), không rơi vào lỗi ext chung
- **Guard decompression bomb**: `MaxImagePixels` 50 MP · `MaxImageDimension` 20000px, đọc header bằng `image.DecodeConfig` và chặn TRƯỚC `image.Decode`. Áp cho MỌI nhánh kể cả passthrough/`ModeRaw`/`.gif`
- **Passthrough là pixel-first**: chỉ giữ nguyên khi ảnh **vừa hộp** VÀ file nhẹ (`SkipBelowKB`). `ModeFill` **luôn xử lý** — không cần mẹo `SkipBelowKB: -1` nữa. `.gif` luôn passthrough (giữ animation) và thắng cả `ModeFill`
- Semaphore max 4 concurrent (tránh CPU spike)
- WebP output ≥ gốc → giữ gốc, **chỉ khi** ảnh gốc đã vừa hộp

### Semaphore: kiểm `ctx.Err()` TRƯỚC `select`
Go chọn **ngẫu nhiên** giữa các case cùng sẵn sàng. Hàng đợi còn chỗ ⇒ cả 2 case ready ⇒ request đã huỷ vẫn có xác suất được nhận việc (và test đỏ/xanh thất thường).
```go
if err := ctx.Err(); err != nil { return err }   // ✅ bắt buộc
select {
case sem <- struct{}{}: return nil
case <-ctx.Done():      return ctx.Err()
}
```
`defer releaseSlot()` chỉ đăng ký **sau** khi acquire trả nil — đặt trước sẽ nhả một slot chưa hề chiếm.

## Lưu ý
- **libwebp** phải cài trên server (CGO): macOS `brew install webp`, Ubuntu `apt install libwebp-dev`
- `Store`/`PrivateStore` wire trong `internal/bootstrap/app.go` (`storage.New`)

## ⚠ HandleFormImage KHÔNG tự xoá ảnh cũ — bug pattern đã gặp (branch, company logo/icon 2026-07-16)

`imaging.HandleFormImage(c, h.Store, "fileImage", "folder/", &item.Image, nil)` chỉ **ghi đè** `*target` khi có file mới upload. Nó **không hề biết** giá trị cũ, nên không xoá được:
- Ảnh cũ trên S3 khi user thay ảnh mới (record → mất tham chiếu, ảnh cũ mồ côi mãi trên S3)
- Ảnh cũ trong DB + S3 khi user bấm nút xoá ảnh mà không upload ảnh thay thế (FE gửi field rỗng, nhưng nếu BE không đọc field đó → DB giữ nguyên giá trị cũ, S3 không đổi gì)

**Checklist bắt buộc khi viết/sửa handler Update có upload ảnh (multipart):**

1. **Capture giá trị cũ trước khi bind field mới:**
   ```go
   var item xxxDao.Xxx
   h.DB.First(&item, id)
   oldImage := item.Image // BẮT BUỘC lấy trước khi bindXxxFormFields ghi đè
   ```
2. **FE phải gửi tín hiệu "xoá ảnh" tường minh** — 1 form control mang giá trị ảnh hiện tại (khởi tạo từ `model.image`), set về `null` khi user bấm nút xoá. `toFormData()` serialize `null` → `''`. Nếu field này không tồn tại trong `FormGroup`, `patchValue` sẽ **âm thầm bỏ qua** (không lỗi, không cảnh báo) — đây là root cause thật sự của bug, không phải lỗi BE.
3. **BE phân biệt "field không gửi" vs "gửi rỗng"** bằng `c.GetPostForm(key)` (trả `ok bool`), KHÔNG dùng `c.PostForm(key) != ""` (không phân biệt được 2 case):
   ```go
   if v, ok := c.GetPostForm("image"); ok && v == "" {
       item.Image = "" // user bấm xoá, không upload ảnh mới
   }
   ```
4. **Sau khi Save/Update DB thành công**, gọi helper dùng chung `imaging.DeleteOldIfChanged` (KHÔNG tự viết lại strip-prefix mỗi module — đã có sẵn trong `pkg/imaging/gin_helper.go`):
   ```go
   imaging.DeleteOldIfChanged(h.Store, oldImage, item.Image) // best-effort, so theo KEY (ToKey), no-op nếu oldImage=="" hoặc không đổi
   ```
5. **FE: nếu chỉ xoá ảnh (không chọn file mới)**, phải ép request đi multipart (`isToFormData.set(true)`) — nếu logic FE chỉ bật FormData khi có file mới chọn (`if (this.file) { isToFormData.set(true) }`), request sẽ đi JSON và field xoá ảnh không bao giờ tới được BE dù đã làm đúng bước 2–4.
6. **Delete record** (không chỉ Update) — load `item` trước khi xoá record, gọi `imaging.DeleteOldIfChanged(h.Store, item.Image, "")` sau khi xoá DB thành công. Ảnh của record bị xoá hẳn cũng phải dọn theo, không chỉ ảnh bị thay khi update.

**Resource đã audit + fix đủ 6 bước (2026-07-16):** `config/branch` (`image`), `config/company` (`logo_image`, `icon_image`), `config/service_type` (`image`, thêm field `Image *string` vào update-input struct để phân biệt nil/rỗng), `config/service` (`image`, qua map `Updates` — GORM map update ghi cả zero-value nên field `image:""` tự xoá được, chỉ thiếu bước dọn S3), `config/user_profile` (`avatar`, cả `UpdateUserProfile` lẫn `CreateAvatarUser`), `crm/customer` (`avatar` — chỉ có luồng replace qua `CreateAvatar`, không có nút xoá riêng), `loyalty/coupon` (`imageCoupon` — `UpdateCoupon`, `DeleteCoupon`, `MultiDeleteCoupon`), `zalo/game` (campaign + gift `image` — DB clear vốn đã đúng vì bind trực tiếp qua `form:"image"` tag, chỉ thiếu dọn S3), `zalo/news` (post `thumbnail`, promotion + slide `image` — cùng dạng với zalo/game), `config/referral_card` (bug SẴN CÓ: `Delete()` gọi với full CDN URL thay vì S3 key → xoá luôn silently fail, đã fix dùng `DeleteOldIfChanged`).

**Không cần fix (ngoài phạm vi checklist này):** `config/facebook_page` (không có field ảnh trên entity, `UploadFacebookImage` là uploader đa file cho chat attachment), `crm/tinymce` (upload ảnh inline nhúng rich-text, không phải field ảnh 1-record-1-ảnh có thể "thay/xoá").

Module có ảnh nhưng CHƯA audit (ngoài scope lần này, cần check khi đụng vào): treatment_course attachment, EMR cls_result, các module khác dùng `ProcessAndUpload`/`SaveUploadedFile` trực tiếp ngoài 2 pattern trên — `grep -rn "ProcessAndUpload\|HandleFormImage" backend/internal/modules` để liệt kê lại danh sách hiện tại.

## Temp file trên server — phải xoá sau khi upload S3 xong

Nếu flow cần lưu file tạm trên disk server trước khi đẩy lên S3 (không upload thẳng từ multipart stream), PHẢI đảm bảo file tạm được xoá **dù thành công hay lỗi**:

- Driver local ghi qua file tạm cùng thư mục + `rename` (`pkg/storage/local.go`), `defer os.Remove(tmp)` — không bao giờ để file ghi dở dưới đúng key. (`SaveToTmpThenS3` cũ đã gỡ 2026-10-19.)
- `pkg/imaging/imaging.go` (dùng `os.CreateTemp` khi xử lý ảnh lớn): `defer os.Remove(tmp.Name())` ngay sau khi tạo file.
- Export file tồn tại có thời hạn (không xoá ngay) → dùng `pkg/cleanup.ScheduleFileDelete(filePath, after)` (`time.AfterFunc` xoá sau N phút/giờ, có log nếu xoá fail), KHÔNG tự viết `time.Sleep` + `os.Remove` rải rác.
- KHÔNG bao giờ `os.Create`/lưu file tạm mà thiếu `defer os.Remove(...)` ngay dòng kế tiếp — nếu có return/panic giữa chừng, file rác tồn tại vĩnh viễn trên disk server.
