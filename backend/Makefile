# Backend make targets — chạy từ backend/

.PHONY: test test-integration test-mem test-scale

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------
# ⚠ `go test ./...` KHÔNG chạy integration test (chúng có build tag `integration`).
# Suite integration là lưới an toàn thật của tầng export/service — chạy nó trước khi tin "đã xanh".

# Test thuần, không cần DB. Nhanh (~vài giây).
test:
	go test ./...

# Test integration — CẦN MySQL dùng-một-lần.
#
# ⛔ TEST_MYSQL_DSN phải trỏ DB có đuôi `_test`/`_tmp`. TUYỆT ĐỐI KHÔNG trỏ vào DB_NAME trong .env:
#    `testhelpers.New` gọi dropAllTables = DROP MỌI BẢNG. Mô hình DB-per-customer ⇒ mất sạch dữ liệu
#    của một khách. Xem skill/skills/writing-mysql-migrations/SKILL.md mục ⛔.
#
# Ví dụ:
#   mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS cluvix_ci_test"
#   make test-integration TEST_MYSQL_DSN='root:pass@(localhost:3306)/cluvix_ci_test?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true'
#
# ⚠ Thiếu TEST_MYSQL_DSN thì suite SKIP nhưng `go test` VẪN in `ok` — XANH GIẢ. Target này chặn
# trường hợp đó bằng cách fail ngay nếu biến trống.
test-integration:
	@test -n "$(TEST_MYSQL_DSN)" || (echo "ERROR: cần TEST_MYSQL_DSN trỏ DB dùng-một-lần (_test/_tmp). Xem chú thích trong Makefile." && exit 1)
	@case "$(TEST_MYSQL_DSN)" in *_test*|*_tmp*) ;; *) echo "ERROR: TEST_MYSQL_DSN phải trỏ DB đuôi _test/_tmp — từ chối chạy để khỏi DROP nhầm DB thật." && exit 1;; esac
	TEST_MYSQL_DSN='$(TEST_MYSQL_DSN)' go test -tags=integration -count=1 -p 1 -timeout 40m ./...

# Phép đo RAM của nhánh export dựng-file-trong-bộ-nhớ (excelize). Không cần DB.
# Tốn vài giây + tới ~1,5GB heap ở ca 200k dòng — nên phải bật tường minh, không chạy mặc định.
test-mem:
	EXPORT_MEM_PROBE=1 go test -count=1 -v -run 'TestExcelizeMemoryPerRow|TestExcelizeStreamWriterMemory' ./pkg/export/

# Bench nghiệm thu quy mô của tầng export — CẦN bench đã nạp sẵn ~524k dòng order_service_used.
# Công thức dựng bench: bmad/export-single-pass-cursor/stories/story-05-scale-acceptance.md §Dev Notes.
# ⚠ EXPORT_BENCH_DSN cũng phải là DB đuôi _test/_tmp (test tự chặn nếu không).
test-scale:
	@test -n "$(EXPORT_BENCH_DSN)" || (echo "ERROR: cần EXPORT_BENCH_DSN trỏ DB bench (_test/_tmp)." && exit 1)
	EXPORT_BENCH_DSN='$(EXPORT_BENCH_DSN)' go test -tags=integration -count=1 -p 1 -timeout 40m -v -run 'TestScale_' ./pkg/export/


# Thông báo license của các Go module được biên dịch vào binary — chạy khi build bản phát hành
# và đóng gói THIRD_PARTY_NOTICES.txt kèm binary (xem NOTICE mục "other dependencies").
.PHONY: third-party-notices
third-party-notices:
	bash scripts/third-party-notices.sh THIRD_PARTY_NOTICES.txt

# ---------------------------------------------------------------------------
# Build phát hành (Linux) — binary server + migrate kèm THIRD_PARTY_NOTICES.txt cùng thư mục.
# ---------------------------------------------------------------------------
# Chạy trên máy build Linux: go-webp dùng cgo + libwebp nên KHÔNG cross-compile từ macOS được.
# ⚠ KHÔNG đặt DIST trong backend/tmp/ (thư mục binary của air) và KHÔNG chạy khi đang dev cùng air:
#   thư mục mới ở gốc backend/ kích air rebuild. Mặc định ra dist/ (đã .gitignore).
DIST ?= dist
GOOS_RELEASE ?= linux
GOARCH_RELEASE ?= amd64

.PHONY: build-linux
build-linux:
	mkdir -p $(DIST)
	GOOS=$(GOOS_RELEASE) GOARCH=$(GOARCH_RELEASE) go build -trimpath -o $(DIST)/cluvix-crm .
	GOOS=$(GOOS_RELEASE) GOARCH=$(GOARCH_RELEASE) go build -trimpath -o $(DIST)/migrate ./cmd/migrate
	bash scripts/third-party-notices.sh $(DIST)/THIRD_PARTY_NOTICES.txt

# Chuyển file đính kèm khách CŨ (attachment.file_url là URL CDN public) sang kho private — chạy 1 lần.
# Mặc định DRY-RUN. Chạy thật: make attachment-privatize ARGS='--apply' (hỏi gõ lại tên DB);
# rồi sau khi kiểm: ARGS='--delete-public-from <manifest.csv>'. Xem DEPLOY.md mục 2.2.
.PHONY: attachment-privatize
attachment-privatize:
	go run ./cmd/attachment-privatize $(ARGS)
