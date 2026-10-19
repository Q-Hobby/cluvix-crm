package handler

import (
	"golang.org/x/sync/singleflight"
)

// FE mở module dashboard là bắn 21 request SONG SONG. Mỗi endpoint tự cache Redis 60–300s, nhưng
// cache LẠNH thì mọi request cùng thấy miss và cùng tính — và các endpoint dùng CHUNG một truy vấn
// (`getIsAcceptedIDs`; trước 2026-10-19 còn funnel marketing + nguồn online, đã gỡ) thì chi phí nhân theo
// số endpoint, rồi nhân tiếp theo số người đang mở màn. Đo epic `crm-dashboard-scale` (2026-09-08):
// 1 → 3 → 5 user đồng thời = 10,0M → 30,1M → 50,2M dòng InnoDB, tuyến tính ĐÚNG bội số user.
//
// `dashboardSF` gộp các lần tính TRÙNG KHOÁ đang chạy đồng thời trong CÙNG tiến trình thành MỘT.
// Đây là lưới cho cửa sổ cache lạnh, KHÔNG thay cache: cache lo lần sau, singleflight lo lúc này.
//
// ⚠ Khoá phải là ĐÚNG khoá cache. Khoá cache của dashboard đi qua `tenantCacheKey(c)` (gồm
// branchActive + toàn bộ branchIds đã sort) nên hai người khác phạm vi
// KHÔNG bao giờ chung khoá — dùng lại chính khoá đó thì singleflight thừa hưởng nguyên tính đúng
// đắn ấy. Tự chế khoá riêng ở đây là mở đường rò dữ liệu chéo chi nhánh.
//
// ⚠ Hàm bên trong KHÔNG được ghi thẳng response (`pagination.FailQuery(c, …)`): chỉ MỘT request
// chạy nó, nên lỗi sẽ rơi vào response của người dẫn còn những người chờ im lặng nhận giá trị
// rỗng. Trả `error` ra ngoài để TỪNG caller tự trả lỗi trên context của chính mình.
var dashboardSF singleflight.Group

// sfDo chạy fn một lần cho mỗi khoá đang bay; mọi caller trùng khoá nhận cùng kết quả VÀ cùng lỗi.
func sfDo[T any](key string, fn func() (T, error)) (T, error) {
	v, err, _ := dashboardSF.Do(key, func() (any, error) {
		res, e := fn()
		return res, e
	})
	if err != nil {
		var zero T
		return zero, err
	}
	res, ok := v.(T)
	if !ok {
		var zero T
		return zero, nil
	}
	return res, nil
}
