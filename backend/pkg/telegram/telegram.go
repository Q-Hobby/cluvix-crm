package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client — Telegram bot client với worker pattern: 1 goroutine drain queue,
// tránh spawn N goroutines khi error burst + có timeout + report lỗi qua callback.
type Client struct {
	botToken string
	http     *http.Client
	queue    chan job
	onError  func(error)
	pacing   time.Duration
}

type job struct {
	chatID string
	text   string
}

// Option — functional option cho New().
type Option func(*Client)

// WithOnError set callback nhận lỗi (network / HTTP non-200 / queue full).
// Callback CHẠY TRONG WORKER GOROUTINE — KHÔNG được gọi zap/logger gây recursion;
// chỉ nên log ra stderr hoặc file riêng.
func WithOnError(fn func(error)) Option {
	return func(c *Client) { c.onError = fn }
}

// WithPacing đặt khoảng nghỉ giữa 2 message để né rate limit Telegram
// (limit ~30 msg/s/group, 1 msg/s/user). Default 50ms (~20 msg/s).
func WithPacing(d time.Duration) Option {
	return func(c *Client) { c.pacing = d }
}

// WithQueueSize đặt capacity queue. Vượt → drop message + report.
func WithQueueSize(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.queue = make(chan job, n)
		}
	}
}

// WithTimeout đặt HTTP timeout cho mỗi request Telegram. Default 10s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

func New(botToken string, opts ...Option) *Client {
	c := &Client{
		botToken: botToken,
		http:     &http.Client{Timeout: 10 * time.Second},
		queue:    make(chan job, 256),
		pacing:   50 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	go c.worker()
	return c
}

// SendToGroup non-blocking enqueue. Drop nếu queue đầy → backpressure tránh OOM.
func (c *Client) SendToGroup(groupID, text string) {
	c.enqueue(groupID, text)
}

// SendToUser non-blocking enqueue.
func (c *Client) SendToUser(userID, text string) {
	c.enqueue(userID, text)
}

// SendToGroupSync blocking send — dùng cho fatal/panic trước khi os.Exit.
// Bypass queue (app sắp die, không kịp drain), nhưng vẫn có timeout từ http.Client.
func (c *Client) SendToGroupSync(groupID, text string) {
	if c.botToken == "" || groupID == "" || text == "" {
		return
	}
	c.send(groupID, text)
}

func (c *Client) enqueue(chatID, text string) {
	if c.botToken == "" || chatID == "" || text == "" {
		return
	}
	select {
	case c.queue <- job{chatID, text}:
	default:
		c.report(fmt.Errorf("queue full, dropped message (chat=%s)", chatID))
	}
}

// worker drain queue sequential — 1 goroutine duy nhất, không spawn theo message.
// Mỗi send bọc recover riêng — 1 message panic không kill worker, các message sau vẫn chạy.
func (c *Client) worker() {
	for j := range c.queue {
		c.safeSend(j)
		if c.pacing > 0 {
			time.Sleep(c.pacing)
		}
	}
}

func (c *Client) safeSend(j job) {
	defer func() {
		if r := recover(); r != nil {
			c.report(fmt.Errorf("send panic: %v", r))
		}
	}()
	c.send(j.chatID, j.text)
}

// send POST tới Telegram API. KHÔNG dùng parse_mode để tránh lỗi escape HTML
// khi message chứa `<`, `>`, `&` chưa escape — error log thường chứa stack trace.
func (c *Client) send(chatID, text string) {
	// Telegram giới hạn 4096 ký tự / message — truncate phòng tin nhắn quá dài.
	if r := []rune(text); len(r) > 4000 {
		text = string(r[:4000]) + "\n…(truncated)"
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)
	resp, err := c.http.PostForm(apiURL, url.Values{
		"chat_id":                  {chatID},
		"text":                     {text},
		"disable_web_page_preview": {"true"},
	})
	if err != nil {
		// Mask bot token trước khi report — Go HTTP error chứa full URL kèm token.
		c.report(fmt.Errorf("network: %s", c.maskToken(err.Error())))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return
	}

	// Parse Telegram error body
	var r struct {
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(body, &r)

	if resp.StatusCode == 429 && r.Parameters.RetryAfter > 0 {
		// Honor rate limit: sleep để các message tiếp theo không bị reject.
		// Drop message hiện tại — re-enqueue sẽ amplify burst.
		c.report(fmt.Errorf("rate limited, retry_after=%ds: %s", r.Parameters.RetryAfter, r.Description))
		time.Sleep(time.Duration(r.Parameters.RetryAfter) * time.Second)
		return
	}

	c.report(fmt.Errorf("HTTP %d: %s", resp.StatusCode, r.Description))
}

func (c *Client) report(err error) {
	if c.onError != nil {
		// Recover phòng callback panic — không cho stop worker.
		defer func() { _ = recover() }()
		c.onError(err)
	}
}

// maskToken thay bot token trong message bằng "***" để tránh leak khi log error.
// HTTP errors trong Go chứa full URL (bao gồm path /bot<token>/...).
func (c *Client) maskToken(s string) string {
	if c.botToken == "" {
		return s
	}
	return strings.ReplaceAll(s, c.botToken, "***")
}
