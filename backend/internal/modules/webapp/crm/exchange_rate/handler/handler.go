// Package handler — Exchange rate scraping qua currency.world (60s TTL in-memory cache).
// Plan B.2 (2026-05-16): Layered module, handler-only. Stateless ngoài cache trong package.
package handler

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

type Deps struct{}

type Handler struct{}

func New(d Deps) *Handler { return &Handler{} }

// In-memory cache — 60s TTL match PHP behaviour.
var (
	exchangeCache   = map[string]exchangeEntry{}
	exchangeCacheMu sync.Mutex
)

type exchangeEntry struct {
	data      gin.H
	expiresAt time.Time
}

// currencyCodePattern — mã tiền tệ ISO 4217 đúng 3 chữ cái. Tham số `from`/`to` được nội suy thẳng
// vào URL của dịch vụ ngoài nên PHẢI validate trước: không chặn thì client gửi được chuỗi có `/`, `?`,
// `#` để bẻ path/query của request đi ra ngoài (host cố định nên không SSRF sang host khác, nhưng vẫn
// gọi sai endpoint và làm hỏng cache key). `num` cũng phải là số thật, không phải chuỗi tuỳ ý.
var currencyCodePattern = regexp.MustCompile(`^[A-Za-z]{3}$`)

// ExchangeRateCurrency GET /crm/exchange-rate/currency?from=USD&num=1&to=VND
func (h *Handler) ExchangeRateCurrency(c *gin.Context) {
	from := strings.ToUpper(c.DefaultQuery("from", "USD"))
	to := strings.ToUpper(c.DefaultQuery("to", "VND"))
	num := c.DefaultQuery("num", "1")

	if !currencyCodePattern.MatchString(from) || !currencyCodePattern.MatchString(to) {
		response.Validate(c, i18n.T(c, "invalid_data"))
		return
	}
	numValue, err := strconv.ParseFloat(num, 64)
	if err != nil || numValue < 0 {
		response.Validate(c, i18n.T(c, "invalid_data"))
		return
	}
	// Chuẩn hoá lại từ giá trị đã parse — loại mọi ký tự lạ mà `ParseFloat` bỏ qua (`+1`, `1e2`...)
	// khỏi URL đi ra ngoài, đồng thời gộp các biến thể cùng giá trị về 1 cache key.
	num = strconv.FormatFloat(numValue, 'f', -1, 64)

	cacheKey := fmt.Sprintf("%s_%s/%s", from, num, to)

	exchangeCacheMu.Lock()
	if entry, ok := exchangeCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		exchangeCacheMu.Unlock()
		c.JSON(http.StatusOK, entry.data)
		return
	}
	exchangeCacheMu.Unlock()

	url := fmt.Sprintf("https://currency.world/convert/%s_%s/%s", from, num, to)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; .NET CLR 1.1.4322)")

	resp, err := client.Do(req)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	html := string(body)

	data := gin.H{
		"giatri":          extractByID(html, "xrate0"),
		"giatrichuyendoi": extractInputValue(html, "amountv1"),
		"giatringuoclai":  extractByID(html, "xrate1"),
	}

	exchangeCacheMu.Lock()
	exchangeCache[cacheKey] = exchangeEntry{data: data, expiresAt: time.Now().Add(60 * time.Second)}
	exchangeCacheMu.Unlock()

	c.JSON(http.StatusOK, data)
}

// extractByID lấy text content của element có id=xxx (giống PHP find('#xxx').plaintext)
func extractByID(html, id string) string {
	re := regexp.MustCompile(`(?i)id="` + id + `"[^>]*>([^<]*)`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractInputValue lấy value của input có id=xxx (giống PHP find('#xxx').value)
func extractInputValue(html, id string) string {
	re := regexp.MustCompile(`(?i)id="` + id + `"[^>]*value="([^"]*)"`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Pattern ngược: value trước id
	re2 := regexp.MustCompile(`(?i)value="([^"]*)"[^>]*id="` + id + `"`)
	m2 := re2.FindStringSubmatch(html)
	if len(m2) > 1 {
		return strings.TrimSpace(m2[1])
	}
	return ""
}
