package handler

import (
	"math/rand"
	"testing"
)

// walkAllPages duyệt mọi trang cho tới khi hết nhóm, trả về danh sách [start,end) từng trang.
// Có trần vòng lặp để test FAIL (chứ không treo) nếu scheduleGroupPageWindow không tiến được.
func walkAllPages(t *testing.T, counts []int64, perPage int) [][2]int {
	t.Helper()
	var pages [][2]int
	total := len(counts)
	for p := 1; p <= total+2; p++ {
		start, end := scheduleGroupPageWindow(counts, p, perPage)
		if start >= total {
			return pages
		}
		if end <= start {
			t.Fatalf("trang %d không tiến được: [%d,%d)", p, start, end)
		}
		pages = append(pages, [2]int{start, end})
	}
	t.Fatalf("không kết thúc sau %d trang — biên trang không tiến", total+2)
	return nil
}

// Bất biến quan trọng nhất: gộp mọi trang phải phủ ĐÚNG toàn bộ nhóm, liền mạch, không trùng
// không sót. Đây là lưới chặn "trả thiếu im lặng" — trần dòng chỉ được đổi CHỖ CẮT trang,
// tuyệt đối không được làm rơi nhóm nào.
func assertPagesCoverAll(t *testing.T, counts []int64, perPage int) [][2]int {
	t.Helper()
	pages := walkAllPages(t, counts, perPage)
	prev := 0
	for i, pg := range pages {
		if pg[0] != prev {
			t.Fatalf("trang %d bắt đầu ở %d, mong đợi %d (thủng/chồng lấn)", i+1, pg[0], prev)
		}
		if n := pg[1] - pg[0]; n > perPage {
			t.Fatalf("trang %d có %d nhóm, vượt perPage=%d", i+1, n, perPage)
		}
		prev = pg[1]
	}
	if prev != len(counts) {
		t.Fatalf("mọi trang chỉ phủ %d/%d nhóm — có nhóm bị rơi", prev, len(counts))
	}
	return pages
}

func TestScheduleGroupPageWindow_KhongDoiHanhViKhiDuoiTran(t *testing.T) {
	// Ngày đông nhất của tải thật hiện tại: 1.064 lịch / 15 chi nhánh, trang 1 = 629 dòng < 800.
	counts := []int64{120, 95, 88, 80, 76, 60, 58, 52, 48, 45, 40, 38, 35, 30, 25}
	pages := assertPagesCoverAll(t, counts, 8)
	if pages[0] != [2]int{0, 8} {
		t.Fatalf("dưới trần thì trang 1 phải gom đủ perPage=8 nhóm như trước, nhận %v", pages[0])
	}
}

func TestScheduleGroupPageWindow_CatTrangNhoLaiKhiVuotTran(t *testing.T) {
	counts := []int64{818, 635, 619, 615, 567, 327, 299, 139} // phân bố thật ca extreme
	pages := assertPagesCoverAll(t, counts, 8)
	if len(pages) < 2 {
		t.Fatalf("tổng %d dòng phải bị cắt thành nhiều trang, nhận %d trang", 4019, len(pages))
	}
	for i, pg := range pages {
		var rows int64
		for _, c := range counts[pg[0]:pg[1]] {
			rows += c
		}
		// Trang nhiều hơn 1 nhóm thì tổng dòng phải nằm trong trần.
		if pg[1]-pg[0] > 1 && rows > scheduleGroupPageRowBudget {
			t.Fatalf("trang %d có %d dòng, vượt trần %d", i+1, rows, scheduleGroupPageRowBudget)
		}
	}
}

func TestScheduleGroupPageWindow_NhomToHonTranVanTraNguyenVen(t *testing.T) {
	// Một chi nhánh 5.000 lịch: KHÔNG được cắt bớt, phải đứng một mình nguyên vẹn trong trang.
	counts := []int64{5000, 10, 20}
	pages := assertPagesCoverAll(t, counts, 8)
	if pages[0] != [2]int{0, 1} {
		t.Fatalf("nhóm vượt trần phải đứng một mình ở trang 1, nhận %v", pages[0])
	}
}

func TestScheduleGroupPageWindow_TrangVuotCuoiTraRong(t *testing.T) {
	counts := []int64{10, 20, 30}
	start, end := scheduleGroupPageWindow(counts, 99, 8)
	if start != len(counts) || end != len(counts) {
		t.Fatalf("trang vượt cuối phải trả cửa sổ rỗng ở cuối, nhận [%d,%d)", start, end)
	}
}

func TestScheduleGroupPageWindow_KhongCoNhom(t *testing.T) {
	start, end := scheduleGroupPageWindow(nil, 1, 8)
	if start != 0 || end != 0 {
		t.Fatalf("không có nhóm phải trả [0,0), nhận [%d,%d)", start, end)
	}
}

// Fuzz nhẹ: bất biến phủ-đủ phải đúng với mọi phân bố, gồm cả nhóm khổng lồ xen nhóm bé.
func TestScheduleGroupPageWindow_PhuDuVoiPhanBoNgauNhien(t *testing.T) {
	rng := rand.New(rand.NewSource(20260906))
	for iter := 0; iter < 300; iter++ {
		n := rng.Intn(40) + 1
		counts := make([]int64, n)
		for i := range counts {
			if rng.Intn(10) == 0 {
				counts[i] = int64(rng.Intn(4000) + 1) // nhóm khổng lồ
			} else {
				counts[i] = int64(rng.Intn(300) + 1)
			}
		}
		perPage := rng.Intn(50) + 1
		assertPagesCoverAll(t, counts, perPage)
	}
}
