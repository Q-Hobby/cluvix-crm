// Package handler — Attachment CRUD.
//
// Quản lý file đính kèm đa đối tượng (customer, appointment...).
// Mọi attachment đều phải biết customer_id — BE tự resolve nếu không có.
//
// File đính kèm là RIÊNG TƯ (user chốt 2026-10-19): lưu ở kho PRIVATE, cột `file_url` chứa KEY
// (đường dẫn tương đối trong kho), KHÔNG có URL công khai. Tải về CHỈ qua
// GET /crm/attachment/:id/download (JWT + perm viewCustomer + scope chi nhánh + scope telesale).
// Bản ghi cũ có `file_url` là URL tuyệt đối (trước 2026-10-19, object public-read) ⇒ download
// redirect 302 tới URL đó (tương thích ngược).
//
// Xem/phát trực tiếp (video/audio tua được, ảnh/PDF xem trước): POST /crm/attachment/:id/signed-url
// (cùng 3 lớp kiểm quyền như download) trả URL KÝ NGẮN HẠN — trình duyệt đọc thẳng kho, không cần JWT.
package handler

import (
	"time"

	"erpCluvix/pkg/storage"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
	// PrivateStore — kho lưu file đính kèm mới.
	PrivateStore storage.Store
	// PublicStore — CHỈ để dọn file đính kèm cũ (URL tuyệt đối, nằm ở kho public) khi xoá bản ghi.
	PublicStore storage.Store
	// SignedURLTTL — hạn URL ký (POST /crm/attachment/:id/signed-url). ≤0 ⇒ storage.DefaultSignedURLTTL.
	SignedURLTTL time.Duration
}

type Handler struct {
	DB           *gorm.DB
	PrivateStore storage.Store
	PublicStore  storage.Store
	SignedURLTTL time.Duration
}

func New(d Deps) *Handler {
	return &Handler{DB: d.DB, PrivateStore: d.PrivateStore, PublicStore: d.PublicStore,
		SignedURLTTL: storage.ClampSignedURLTTL(d.SignedURLTTL)}
}
