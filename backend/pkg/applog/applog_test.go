package applog

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// TestWrite_ChuaInit_KhongPanic — luật số 1: ghi log không được làm hỏng nghiệp vụ. Trong test, CLI,
// hay giai đoạn boot sớm thì applog chưa Init — gọi Write vẫn phải im lặng đi qua.
func TestWrite_ChuaInit_KhongPanic(t *testing.T) {
	Write(Entry{Source: "test", Event: "x", Message: "y"})
	Warn(Entry{Source: "test", Event: "x"})
	Error(Entry{Source: "test", Event: "x"})
	Info(Entry{Source: "test", Event: "x"})
}

// TestToRow_CatMessageQuaDai — `message` là VARCHAR(500) = 500 KÝ TỰ. Không cắt thì MySQL từ chối
// NGUYÊN dòng, tức mất đúng dòng log của sự cố đang cần xem.
func TestToRow_CatMessageQuaDai(t *testing.T) {
	r := toRow(Entry{Message: strings.Repeat("đ", 800)})
	if n := utf8.RuneCountInString(r.Message); n > maxMessageRunes {
		t.Fatalf("message phải bị cắt còn ≤ %d ký tự, đang %d", maxMessageRunes, n)
	}
}

// TestToRow_CatDungBienRune — REGRESSION cho bug 2026-08-16.
//
// Bản đầu cắt theo BYTE (`s[:n]`): chuỗi tiếng Việt bị cắt giữa một rune 3 byte → UTF-8 hỏng → MySQL
// utf8mb4 từ chối cả dòng (lỗi 1366) → mất log. Test cũ không bắt được vì dùng ký tự 2 byte với ngưỡng
// chẵn nên cắt vô tình rơi đúng biên. Ở đây cố ý dùng ký tự **3 byte** và độ dài lẻ để ép sai biên.
func TestToRow_CatDungBienRune(t *testing.T) {
	for _, ch := range []string{"ầ", "ệ", "ỗ", "🙂"} { // 3 byte và 4 byte
		for _, n := range []int{maxMessageRunes + 1, maxMessageRunes + 7, maxMessageRunes*2 + 3} {
			got := toRow(Entry{Message: strings.Repeat(ch, n)}).Message
			if !utf8.ValidString(got) {
				t.Fatalf("cắt %q x%d ra chuỗi UTF-8 KHÔNG hợp lệ — MySQL sẽ vứt cả dòng log", ch, n)
			}
			if c := utf8.RuneCountInString(got); c > maxMessageRunes {
				t.Fatalf("cắt %q x%d còn %d ký tự, vượt trần %d", ch, n, c, maxMessageRunes)
			}
		}
	}
}

// TestTruncate_KhongCatKhiVuaDu — chuỗi ngắn hơn trần phải giữ NGUYÊN VẸN, không mất ký tự cuối.
func TestTruncate_KhongCatKhiVuaDu(t *testing.T) {
	in := strings.Repeat("ầ", maxMessageRunes)
	if got := truncate(in, maxMessageRunes); got != in {
		t.Fatalf("chuỗi đúng bằng trần bị cắt mất: %d ký tự còn %d",
			utf8.RuneCountInString(in), utf8.RuneCountInString(got))
	}
	if got := truncate("abc", 10); got != "abc" {
		t.Fatalf("chuỗi ngắn hơn trần phải giữ nguyên, được %q", got)
	}
}

// TestToRow_LevelMacDinh — quên set Level thì phải là info, không được để chuỗi rỗng (cột NOT NULL và
// mọi câu lọc theo level sẽ trượt).
func TestToRow_LevelMacDinh(t *testing.T) {
	if r := toRow(Entry{Source: "s"}); r.Level != LevelInfo {
		t.Fatalf("Level mặc định phải là %q, được %q", LevelInfo, r.Level)
	}
}

// TestToRow_ChuoiRong_ThanhNULL — ref_type/request_id/error_code rỗng phải vào DB là NULL, KHÔNG phải
// chuỗi rỗng: `WHERE error_code IS NULL` là câu người ta sẽ viết, và ” làm hỏng cả GROUP BY.
func TestToRow_ChuoiRong_ThanhNULL(t *testing.T) {
	r := toRow(Entry{Source: "s"})
	if r.RefType != nil || r.RequestID != nil || r.ErrorCode != nil {
		t.Fatalf("chuỗi rỗng phải thành NULL, được ref_type=%v request_id=%v error_code=%v",
			r.RefType, r.RequestID, r.ErrorCode)
	}
	r2 := toRow(Entry{ErrorCode: "-216"})
	if r2.ErrorCode == nil || *r2.ErrorCode != "-216" {
		t.Fatalf("error_code có giá trị thì phải giữ nguyên, được %v", r2.ErrorCode)
	}
}

// TestToRow_DataThanhJSONHopLe — cột `data` là JSON: ghi vào phải parse lại được.
func TestToRow_DataThanhJSONHopLe(t *testing.T) {
	r := toRow(Entry{Data: map[string]any{"account_id": 4, "zalo_user": "u1"}})
	var back map[string]any
	if err := json.Unmarshal(r.Data, &back); err != nil {
		t.Fatalf("data phải là JSON hợp lệ: %v", err)
	}
	if back["zalo_user"] != "u1" {
		t.Fatalf("mất dữ liệu trong data: %+v", back)
	}
}

// TestToRow_DataQuaTo_KhongNemCaDong — payload khổng lồ (dump cả response) không được làm mất luôn
// dòng log; giữ dấu vết để người đọc biết là đã bị cắt.
func TestToRow_DataQuaTo_KhongNemCaDong(t *testing.T) {
	r := toRow(Entry{
		Source: "s",
		Data:   map[string]any{"blob": strings.Repeat("x", maxDataBytes+10)},
	})
	if len(r.Data) == 0 || !strings.Contains(string(r.Data), "_truncated") {
		t.Fatalf("data quá to phải thành dấu vết _truncated, được %q", string(r.Data))
	}
	if r.Source != "s" {
		t.Fatal("phần còn lại của dòng log phải giữ nguyên")
	}
}

// TestClose_ChuaInit_KhongPanic — Close phải an toàn khi chưa từng Init (CLI, test, hoặc boot lỗi sớm
// rồi chạy thẳng vào shutdown).
func TestClose_ChuaInit_KhongPanic(t *testing.T) {
	Close(time.Second)
	Close(time.Second) // gọi 2 lần cũng không được panic
}

// TestWrite_SongSongVoiClose_KhongPanic — REGRESSION cho race tự tạo khi thêm Close (2026-08-16):
// bản đầu lấy channel ra khỏi RLock rồi mới gửi ⇒ Close() đóng channel đúng khe đó ⇒ "send on closed
// channel" ⇒ panic giữa nghiệp vụ. Chạy với `-race` để có giá trị thật.
func TestWrite_SongSongVoiClose_KhongPanic(t *testing.T) {
	Init(nil) // DB nil → Init là no-op; Write/Close vẫn phải chịu được gọi đồng thời
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Write(Entry{Source: "race", Event: "x", Message: "y"})
		}()
	}
	go Close(time.Second)
	wg.Wait()
}
