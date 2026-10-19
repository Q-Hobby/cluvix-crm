package handler

import (
	"time"

	noteDao "erpCluvix/internal/modules/webapp/crm/note/dao"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/define"
)

// canDeleteNote kiểm tra user có được phép xóa note không.
//
// Logic (mirror FE checkDeleteNote):
//   - Có quyền deleteNote → luôn được phép.
//   - Không có quyền deleteNote → phải đủ CẢ 4 điều kiện:
//     1. Có quyền createNote.
//     2. Là người tạo note (created_by = userID) — check trước option để tránh DB query.
//     3. user_option id=6 (OPTION_DELETE_NOTE) = 1 (bật).
//     4. Note được tạo trong ngày hôm nay (theo timezone thương hiệu — pkg/brand).
//
// Query order tối ưu: 1 Redis call → creator check (free) → option DB → tz (in-mem cache).
func (h *Handler) canDeleteNote(userID uint, note *noteDao.Note) bool {
	// 1 Redis call — lấy hết perms, check cả deleteNote lẫn createNote
	perms := h.getPermissions(userID)

	if containsPerm(perms, "deleteNote") {
		return true
	}
	if !containsPerm(perms, "createNote") {
		return false
	}

	// Creator check — free, no DB; non-creator exit trước khi chạm option query
	if note.CreatedBy == nil || uint(*note.CreatedBy) != userID {
		return false
	}

	// Option check — DB (chỉ chạy nếu là creator)
	var optionVal int
	h.DB.Table("user_option").Where("id = ?", define.UserOptionDeleteNote).
		Limit(1).Pluck("options", &optionVal)
	if optionVal != 1 {
		return false
	}

	// Same-day check — brand.Location dùng in-memory cache sau lần đầu. Story-36 (D15): múi giờ của
	// thương hiệu, không đọc company_id của note nữa.
	if note.CreatedAt == nil {
		return false
	}
	now := time.Now().In(brand.Location(h.DB))
	createdAt := time.Unix(*note.CreatedAt, 0).In(now.Location())

	return now.Year() == createdAt.Year() &&
		now.Month() == createdAt.Month() &&
		now.Day() == createdAt.Day()
}

// getPermissions lấy toàn bộ permission list của user (Redis-cached, TTL 1h).
func (h *Handler) getPermissions(userID uint) []string {
	if h.RbacSvc == nil {
		return nil
	}
	perms, _ := h.RbacSvc.GetUserPermissions(userID)
	return perms
}

func containsPerm(perms []string, code string) bool {
	for _, p := range perms {
		if p == code {
			return true
		}
	}
	return false
}
