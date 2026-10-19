# Mutation test — chứng minh test có NEO hành vi

> Tách khỏi `SKILL.md` (2026-09-07) khi file chạm ngưỡng 500 dòng body.

## Mục lục

- [Cách chạy `-overlay`](#mutation-test--chứng-minh-test-có-neo-hành-vi)
- [Các bẫy khi cấy mutation (danh sách gạch đầu dòng bên dưới)](#mutation-test--chứng-minh-test-có-neo-hành-vi)
- [Test "ca xấu" phải dùng ĐÚNG dạng input mà code đọc được](#test-ca-xấu-phải-dùng-đúng-dạng-input-mà-code-đọc-được--không-thì-nó-không-phân-biệt-được-gì)
- [Lưới có thể ĐỎ 0/N vì chính nó hỏng — 3 dạng](#lưới-có-thể-đỏ-0n-vì-chính-nó-hỏng--hai-dạng-đã-đo-2026-09-09)
- [Refactor code nhiều tác dụng phụ — 2 clone + băm mọi bảng](#refactor-code-nhiều-tác-dụng-phụ--2-clone--băm-mọi-bảng)

Test xanh không chứng minh nó neo gì. Cách kiểm duy nhất: đổi 1 hằng số / 1 dấu so sánh trong code
sản phẩm rồi chạy lại — test **phải đỏ**. Đỏ 0/N mutation = cả lớp test đang vô dụng dù coverage đẹp.

```bash
# overlay = ghi đè file ẢO lúc build, working tree KHÔNG bị đụng
echo '{"Replace":{"/abs/path/gate.go":"/abs/path/tmp/gate_mutated.go"}}' > /tmp/ov.json
go test -overlay=/tmp/ov.json ./internal/modules/webapp/<module>/<package>/
```

- **KHÔNG dùng script sửa-file-rồi-khôi-phục**: lệnh bị timeout/kill giữa chừng sẽ để lại code đã
  mutate trong working tree (đã xảy ra). `-overlay` không có chế độ hỏng đó.
  ⚠ Lý do thứ hai, đo được 2026-09-08: `air` của user đang chạy nền và **watch working tree**, nên
  mỗi lần cấy mutation vào file sản phẩm là air rebuild vào code hỏng ⇒ **dev server của user chết**
  suốt thời gian chạy mutation (`backend/tmp/build-errors.log` đầy `exit status 1`). Nó tự hồi phục
  khi khôi phục file, nhưng đó là làm hỏng môi trường của người khác để chạy test của mình.
  `-overlay` ghi đè file ẢO lúc build ⇒ air không thấy gì, server sống nguyên.
- **`require.Nil`/`assert.Nil` của testify KHÔNG phân biệt typed-nil**: dùng `reflect.Value.IsNil()`
  nên `(*SomeStruct)(nil)` bọc trong interface vẫn được coi là nil ⇒ test xanh dù guard `if c == nil`
  đã bị xoá khỏi code sản phẩm. Muốn khoá đúng so sánh interface: `require.True(t, x == nil, ...)`.
- **Mutate CẢ chỗ GỌI (seam), không chỉ hàm thuần** — lớp lỗi tốn nhất của một epic cũ (notification-core-v2, module đã bỏ 2026-10-17):
  40% finding QA (18/45) và 29% nợ còn mở là mutation vào MỐI NỐI domain→builder/component→service/
  event→handler mà suite vẫn xanh, dù tỉ lệ test:sản phẩm 1,44:1. Test từng mảnh riêng không canh chỗ
  ghép. Story tạo mối nối mới (vd domain gọi builder consumer mới, listener gọi service mới) phải có
  ≥1 test đi XUYÊN QUA mối nối đó (mutation ở điểm gọi ⇒ đỏ), không chỉ test 2 đầu riêng lẻ.
- **Hằng-hoá đối số: chọn fixture sao cho KHÔNG hằng nào qua được.** Test seam chỉ có MỘT bước thì
  luôn tồn tại hằng làm mutation xanh — chính giá trị fixture đang dùng (thật: seam truyền
  `batch_size`, fixture 2 dòng ⇒ hằng `2` qua lọt, seam vẫn coi như có lưới). Dựng ≥2 bước với giá trị
  khác nhau, kiểm bằng cách thử đúng hằng khít nhất của từng bước.
- **Đột biến phải COMPILE được mới tính là lưới đỏ.** `go test` fail vì build error là dương tính giả — không chứng minh test bắt được hành vi nào. Build file overlay trước (`go build -overlay=... [-tags ...] ./<pkg>/`); không build được thì bỏ đột biến đó và viết lại, chỉ khi build OK mới đọc kết quả test.
- Test assert lại **chính hằng số nó đang kiểm** (`if cap != define.X`) thì mutation nào cũng qua →
  assert **số tuyệt đối** (`if cap != 300`).
- **Guard SQL bị guard bộ nhớ che**: `claimX` thường có 2 lưới — `if conv.Col != nil` (bộ nhớ) rồi
  `WHERE ... AND col IS NULL` (SQL). Fixture seed giá trị **trước** khi nạp `conv` sẽ dừng ở lưới bộ
  nhớ, câu SQL **không bao giờ chạy** → gỡ `AND col IS NULL` mà cả suite vẫn xanh (thật: 316 test,
  guard sống 4 story). Công thức đúng: **nạp snapshot `conv` TRƯỚC**, seed giá trị vào DB SAU (mô
  phỏng luồng khác chen giữa) — khi đó SQL là lưới DUY NHẤT. Assert thêm tiền đề `conv.Col == nil`
  sau khi nạp, để test không xanh giả nếu helper đổi.
- Nghiệm thu sống trên **đường chính** KHÔNG thay được việc kiểm **phanh** (cooldown, trần, ngắt
  mạch): phanh cố ý hiếm khi bị chạm nên chỉ lộ khi ép chạm. Đợt 2026-08-18: backfill nghiệm thu sống
  "ĐẠT" trong khi cả 3 phanh chết âm thầm — đường chính chạy, không phanh nào từng chạy.

- **Mutate CHÍNH cái lưới vừa dựng, không chỉ code sản phẩm.** Lưới mới viết để bịt một lỗ thường
  cũng có lỗ — 3 ca đã gặp: (a) lưới quét khớp **chuỗi con** (`"IsSync"`) bị lách khi ai đó **đảo**
  điều kiện (`!x` → `x`), phải khớp nguyên cụm; (b) lưới đọc `if` **gần nhất** báo động giả khi lời
  gọi nằm trong `if err := f(); err == nil` (gate thật ở tầng ngoài) — phải gom mọi tầng `if` bao
  quanh; (c) test xanh **do thứ tự seed**: `First()` thêm `ORDER BY id`, seed row ĐÚNG ở id nhỏ hơn
  thì truy vấn thiếu cột lọc vẫn tình cờ trả đúng ⇒ seed row **SAI** ở id nhỏ hơn.
- **Hai test thêm trong cùng một bản vá thường KHÔNG neo cùng một thứ** — mutation-test riêng từng
  cái. Dùng một phép thử cho cả hai là để lại một lưới chưa từng được chứng minh có răng.
- **Mutation phải tái hiện TRUNG THỰC bug gốc.** Cấy nửa vời cho ra con số vô lý rồi tưởng là đã
  chứng minh: bản vá đổi `scanned = len(ids)` → `scanned += len(chunk)`, cấy lại dòng cũ mà GIỮ luôn
  `+=` ⇒ cộng hai lần, báo 110 thay vì 60. Phải cấy đúng hình dạng cũ (gán trước, bỏ `+=`) mới thấy
  đúng **một** test bắt.
- **Vô hiệu hoá một helper TỰ GHI RESPONSE thì mutation không có răng — lưới vẫn xanh giả.** Ca thật
  2026-09-07: bản vá là `id, ok := parseCustomerID(c); if !ok { return }`, mà `parseCustomerID` tự gọi
  `response.NotFound(c)` bên trong. Cấy kiểu quen tay `if false && !ok {` vẫn để **lời gọi parse chạy**
  ⇒ 404 đã được ghi vào `ResponseRecorder`, `w.Code` chốt ở 404 ⇒ test **PASS** và báo sai rằng bản vá
  không load-bearing. Phải bỏ **HẲN** lời gọi (`id := 0; _ = id` rồi trả biến thô vào query) mới đỏ
  đúng 3/7 ca. Quy tắc chung: mutation phải xoá **tác dụng phụ**, không chỉ xoá **luồng điều khiển** —
  helper nào vừa trả giá trị vừa ghi response/log/metric thì `if false &&` là cách cấy SAI.
  ⚠ Biến thể hay đi kèm: bỏ hẳn lời gọi làm biến thành `declared and not used` ⇒ **build fail**.
  Build fail KHÔNG phải là "mutation đỏ" — nó chưa chứng minh test nào bắt được; phải thêm `_ = x`
  cho biên dịch được rồi mới đọc màu của test.
- **AC dạng "KHÔNG xảy ra X" hoặc "mặc định Y khi lỗi" ⇒ BẮT BUỘC mutation-gate trước khi báo Done.**
  Lật mặc định (`v := true` → `false`), bỏ điều kiện, xoá cột khỏi `SELECT` — test phải đỏ. Epic
  zalo-personal-group: 6/9 story dính lớp lỗi này, **cả 6 đều do mutation bắt, 0 lần do review mắt**.
- **Mười hình dạng "test xanh vì lý do sai" đã gặp thật** — kiểm tiền đề trước khi tin màu xanh:

  | Hình dạng | Ví dụ thật |
  |---|---|
  | Fixture làm assertion vô nghĩa | payload sticker có `href` ⇒ `contentText` vốn đã trả `""`, assert `== ""` không kiểm gì |
  | Hàm thoát sớm ở tiền đề | test chạy không DB, `enrichGroup` return ngay ở `m.db == nil` ⇒ gỡ gate `ThreadType` vẫn xanh |
  | Bản giả không bao giờ được gọi | test đếm `profileCalls == 0` xanh vì nhánh đó câm, không phải vì gate chặn |
  | Tiền đề chặn trước khi tới nơi | `context.WithTimeout(300ms)` < `minCallBudget` ⇒ `gatedWrite` trả `ErrNoTimeBudget`, lời gọi chưa từng xảy ra |
  | Assert thứ dễ, không phải thứ cần giữ | assert *tập id* trả về — tập id vẫn đúng khi cột rụng khỏi `SELECT` vì `WHERE` đọc thẳng DB |
  | Không ai giữ nhánh fail-open | code fail-open đúng nhưng lật `true`→`false` không test nào đỏ |
  | Dữ liệu test trùng giá trị cũ | ca "không ghi đè" dùng `[5,2]` trên bản ghi đang là `2`; mutation "lấy min" ra đúng `2` ⇒ xanh dù đã ghi đè. Chọn dữ liệu để **mọi cách sai đều ra khác giá trị cũ** |
  | Các nhánh phân loại có **cùng số dòng** ⇒ hoán vị hằng ra cùng kết quả | seed có đúng 1 lịch `status = 4` và 1 lịch `status = 1`; mutation đổi `COUNT(CASE WHEN status = 4)` thành `status = 1` vẫn ra 1 ⇒ xanh. Mỗi nhánh phải có **số dòng khác nhau** |
  | Test dựng LẠI logic thay vì gọi hàm sản phẩm | test tự build SQL thay vì gọi `buildXxxQuery` của handler ⇒ cấy `GROUP BY`, ép join vô điều kiện, bỏ scope tenant vào code sản phẩm đều XANH cả ba. Test khoá hình dạng SQL phải chạy qua ĐIỂM DỰNG QUERY DUY NHẤT của handler; sau khi sửa: 8/8 mutation đỏ |
  | Input KHÔNG đi qua nổi lớp parse ⇒ cả bản cũ lẫn bản mới xử lý giống nhau | test đa-giá-trị dùng `filter[k][]=1&filter[k][]=2`, dạng mà `getArrayValues` KHÔNG đọc ⇒ hai bên cùng từ chối, mutation không đỏ. Dạng đúng: `filter[k][0]`/`[1]`. Chi tiết ở mục cuối file |

- **Mọi test khẳng định "KHÔNG xảy ra X" phải có VẾ ĐỐI CHỨNG** khẳng định ca ngược **CÓ** xảy ra,
  chạy cùng fixture/DB/bản giả. Không có đối chứng thì số 0 không phân biệt được "gate chặn" với
  "đường code câm".
- **Self-check trong chính test** để nó tự chứng minh đã chạm đường code, trước khi assert:
  ```go
  if len(f.asked) == 0 {
      t.Fatal("chưa hề xin trang nào — test đang xanh vì lý do sai")
  }
  ```
  Rẻ hơn nhiều so với phát hiện ở pass QA sau. Kiểm chính self-check bằng cách tái lập điều kiện cũ.
- **Assert đúng HẬU QUẢ NẶNG nhất, không phải hậu quả dễ thấy.** Gate CSAT đặt sai chỗ (sau bước
  reserve row) thì hậu quả nặng không phải "gửi nhầm phiếu" mà là **chiếm mất row `UNIQUE
  conversation_id` khiến lần sau bị dedup coi là đã gửi** — test chỉ assert "không gửi tin" sẽ bỏ lọt.
## Test "ca xấu" phải dùng ĐÚNG dạng input mà code đọc được — không thì nó không phân biệt được gì

Đo 2026-09-08. Viết test chứng minh `filter[customer_id]` nhiều giá trị bị từ chối, dùng URL
`?filter[customer_id][]=1&filter[customer_id][]=2`. Test **xanh**. Mutation (khôi phục bản lỗi) —
vẫn **xanh**. Bản vá trông như được neo, thật ra test không chạm tới nó.

Lý do: `pkg/query.getArrayValues` chỉ đọc **`filter[key][0]`, `filter[key][1]`…**,
`filter[key][in][0]…`, `key[0]…`, cộng `QueryArray("key[]")` và `QueryArray("key")`. Dạng
`filter[key][]` KHÔNG khớp nhánh nào ⇒ trả rỗng ⇒ **cả bản cũ lẫn bản mới đều từ chối như nhau**.
Test hỏi một câu mà hai bên trả lời giống hệt.

Đổi sang `?filter[customer_id][0]=1&filter[customer_id][1]=2` thì mutation đỏ ĐÚNG một test và chỉ
test đó — cô lập chính xác.

⇒ Rút ra, áp cho mọi test ca-biên: **input phải đi qua được lớp parse tới tận nhánh đang kiểm**.
Cách rẻ để tự bắt: chạy mutation TRƯỚC khi tin test. Mutation không đỏ có hai nghĩa rất khác nhau —
"bản vá không load-bearing" và "test không chạm tới bản vá" — và mặc định phải nghi cái thứ hai
trước, vì nó âm thầm hơn. Cùng họ với các bẫy "lệnh chạy nhưng đo nhầm ca" đã ghi trong
`skills/debugging-backend-queries/reference/perf-list-endpoints.md`.

Đối chứng nên có sẵn trong test đa-giá-trị: seed **hai** khách rồi khẳng định response KHÔNG chứa
khách thứ hai. Nếu bản lỗi trả 200 kèm dữ liệu khách đầu, assertion status một mình đã đủ đỏ; nhưng
có thêm assertion nội dung thì lỗi đọc ra ngay là "nuốt mất khách 2", không phải "sai mã HTTP".
## Lưới có thể ĐỎ 0/N vì chính nó hỏng — hai dạng đã đo (2026-09-09)

Mutation không đỏ, nhưng bản vá vẫn load-bearing. Cả hai lần thủ phạm là TEST, không phải code.

### 1. Bộ đếm neo nhầm callback ⇒ luôn = 0 ⇒ mọi phép so đều đúng

Test "không N+1" đếm query bằng `testDB.Callback().Raw().After("gorm:raw")`. `.Scan()` của GORM đi
qua callback **Query/Row**, còn `Raw` chỉ bắn cho `db.Exec`/`db.Raw` ⇒ biến đếm luôn `0` ⇒ phép so
`five > one` thành `0 > 0` ⇒ xanh vĩnh viễn, kể cả khi cấy hẳn một vòng lặp query vào handler.

```go
// đăng ký CẢ HAI, và phải có đối chứng
testDB.Callback().Query().After("gorm:query").Register(name, bump)
testDB.Callback().Row().After("gorm:row").Register(name, bump)
...
if one == 0 { t.Fatal("bộ đếm = 0 — nó không đếm được gì, mọi kết luận phía dưới vô nghĩa") }
```

⇒ Rule: **mọi test kết luận bằng một CON SỐ phải assert con số đó khác giá trị vô hại trước đã.**

### 2. Ca thử được chọn sao cho bản lỗi và bản đúng trả lời GIỐNG HỆT

Test "không lộ dữ liệu company khác" seed 2 user ở 2 company rồi bỏ company scope ⇒ **vẫn xanh**:
câu thứ hai lọc `WHERE user_id IN (<user công ty mình>)`, user công ty kia không nằm trong danh sách
nên không có gì để rò. Ca rò THẬT là **cùng một `user_id` có dữ liệu ở hai company** — có thật trong
mô hình 1 chủ nhiều phòng khám. Đổi ca thử thì mutation đỏ ngay.

⇒ Rule: khi một lớp bảo vệ (`company_id`) nằm CẠNH một bộ lọc khác đã hẹp sẵn (`user_id IN …`), ca
thử phải làm bộ lọc kia **vô hiệu** thì mới đo được lớp đang kiểm.

### 3. Sink bất đồng bộ: đọc ngay luôn thấy rỗng

`seclog.InitSink` flush theo LÔ 64 entry **hoặc** ticker 2 giây. Test khẳng định "đúng 1 sự kiện"
mà đọc ngay sau request sẽ luôn thấy `0` — và `0 != 1` nên nó đỏ vì lý do sai, hoặc tệ hơn, assert
`<= 1` thì xanh giả. Phải chờ có hạn (poll tới ~4s) rồi mới đếm.

## Refactor code nhiều tác dụng phụ — 2 clone + băm mọi bảng

Tối ưu / đổi thứ tự code có nhiều tác dụng phụ (tính lại số liệu, sổ, kho): clone DB thành 2 bản `*_tmp`, chạy
cách CŨ trên A và cách MỚI trên B với cùng input, rồi băm **mọi bảng** (bỏ cột thời gian) và so A↔B:

```sql
SELECT COUNT(*), BIT_XOR(CAST(CONV(SUBSTRING(SHA2(CONCAT_WS('|', COALESCE(CAST(col1 AS CHAR),'∅'), …),256),1,15),16,10) AS UNSIGNED)) FROM t;
```

- **Bắt buộc đối chứng**: sửa 1 dòng ở B (kiểm `ROW_COUNT() = 1` — KHÔNG chọn bảng rỗng) ⇒ bộ so phải báo đúng bảng đó.
- Nhánh không có dữ liệu trên clone (vd bảng `broker` rỗng) ⇒ ghi rõ là CHƯA được so.

