package handler

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Neo hợp đồng của `sfDo`: N request trùng khoá đang bay ⇒ hàm tính chạy ĐÚNG 1 lần, tất cả nhận
// cùng kết quả. Đây là lưới cho cửa sổ cache lạnh của dashboard (21 request bắn song song, nhiều
// endpoint dùng chung một truy vấn) — đo epic crm-dashboard-scale: 1→3→5 user đồng thời từng cho
// 10,0M → 30,1M → 50,2M dòng InnoDB, tuyến tính đúng bội số user.
func TestSfDo_GopCacLuotTrungKhoaThanhMot(t *testing.T) {
	const n = 32
	var chay atomic.Int64
	batDau := make(chan struct{})

	var wg sync.WaitGroup
	ketQua := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-batDau
			v, err := sfDo("cung-mot-khoa", func() (int, error) {
				chay.Add(1)
				time.Sleep(30 * time.Millisecond) // giữ cửa sổ đủ rộng để mọi goroutine kịp vào
				return 42, nil
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
			}
			ketQua[i] = v
		}(i)
	}
	close(batDau)
	wg.Wait()

	if got := chay.Load(); got != 1 {
		t.Fatalf("hàm tính chạy %d lần, muốn 1 — singleflight KHÔNG gộp", got)
	}
	for i, v := range ketQua {
		if v != 42 {
			t.Fatalf("goroutine %d nhận %d, muốn 42", i, v)
		}
	}
}

// Đối chứng: KHÁC khoá thì KHÔNG được gộp — nếu gộp nhầm là trả dữ liệu của tenant/kỳ khác.
// Không có mốc này thì một `sfDo` hỏng kiểu "luôn dùng chung một khoá" vẫn qua test trên.
func TestSfDo_KhacKhoaThiKhongGop(t *testing.T) {
	const n = 8
	var chay atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, _ := sfDo(string(rune('a'+i)), func() (int, error) {
				chay.Add(1)
				time.Sleep(20 * time.Millisecond)
				return i, nil
			})
			if v != i {
				t.Errorf("khoá %d nhận %d — kết quả bị lẫn giữa các khoá", i, v)
			}
		}(i)
	}
	wg.Wait()
	if got := chay.Load(); got != n {
		t.Fatalf("chạy %d lần cho %d khoá khác nhau — đang gộp nhầm", got, n)
	}
}

// Lỗi phải tới TẤT CẢ người chờ, không chỉ người dẫn. Trước bản vá, hàm tính gọi thẳng
// `pagination.FailQuery(c, …)` nên lỗi chỉ rơi vào response của MỘT request; người chờ nhận giá
// trị rỗng mà HTTP vẫn 200 — hỏng im lặng đúng kiểu khó truy nhất.
func TestSfDo_LoiToiMoiNguoiCho(t *testing.T) {
	const n = 16
	loiMau := errors.New("query hỏng")
	var soLoi atomic.Int64
	var wg sync.WaitGroup
	batDau := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-batDau
			v, err := sfDo("khoa-loi", func() (int, error) {
				time.Sleep(30 * time.Millisecond)
				return 7, loiMau
			})
			if err != nil {
				soLoi.Add(1)
			}
			if v != 0 {
				t.Errorf("lỗi mà vẫn trả %d — caller sẽ dùng số rác", v)
			}
		}()
	}
	close(batDau)
	wg.Wait()
	if got := soLoi.Load(); got != n {
		t.Fatalf("chỉ %d/%d người chờ nhận được lỗi", got, n)
	}
}
