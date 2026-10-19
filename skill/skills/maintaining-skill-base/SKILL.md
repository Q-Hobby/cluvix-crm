---
name: maintaining-skill-base
description: >-
  Bảo trì chính knowledge base này: chạy lại việc tối ưu sau khi knowledge được ghi thêm,
  phân biệt việc phải quét FULL với việc chỉ soi phần MỚI, chốt mốc delta, và 7 gate tự
  kiểm. Dùng skill này KHI được yêu cầu tối ưu/dọn `skill/`, sau một đợt BMAD lớn, sau khi
  restructure module hoặc đổi tên package, hoặc trước khi kết luận "knowledge base đang
  sạch" — vì gate xanh không đồng nghĩa knowledge đúng, và quét theo delta sẽ bỏ lọt đúng
  loại lỗi tốn kém nhất.
---

# Bảo trì knowledge base

> Cấu trúc + luật ghi knowledge: `skill/HOW_TO_UPDATE.md`.
> Lịch sử epic dựng ra cơ chế này: `bmad/done/skill-optimization/` (nếu đã archive).

## Chạy

```bash
skill/skills/_tooling/optimize.sh          # tăng dần (mặc định)
skill/skills/_tooling/optimize.sh --full   # bỏ qua mốc, coi như lần đầu
skill/skills/_tooling/optimize.sh --stamp  # chốt mốc — CHỈ khi mọi thứ đã sạch
```

`/skill-optimize` chỉ là vỏ mỏng gọi script này. **Logic nằm ở script, cố ý** — knowledge
base phục vụ nhiều agent (Claude, Codex, Gemini) và cả người; đặt logic trong slash command
là khoá năng lực bảo trì vào một runtime.

## Hai tầng — KHÔNG gộp được

| | Trigger | Cách quét |
|---|---|---|
| **Môi trường trôi** | Code đổi chỗ/đổi tên. **Không file doc nào bị sửa** mà doc TỰ trở thành sai. | **FULL, mỗi lần.** Rẻ vì là script. |
| **Knowledge mới ghi thêm** | Session sau ghi rule mới: đặt sai chỗ, trùng cái đã có, body phình quá 500 dòng. | **Tăng dần** theo mốc. Đắt vì cần phán đoán. |

Đây là chỗ dễ sai nhất: nếu chỉ soi file đã đổi thì **bỏ lọt 100%** loại lỗi thứ nhất.
Bằng chứng thật: sau restructure module 2026-07-28, 180 đường dẫn trong knowledge trở
thành sai mà `git diff` trên `skills/` trả **rỗng**.

## Mốc delta — `maintaining-skill-base/state.json`

Ghi 4 SHA, hai vai trò khác nhau:
- `skill` · `frontend` · `mobile` — nơi knowledge nằm ⇒ delta = `git diff <sha>` (so với **WORKING
  TREE**, KHÔNG `..HEAD`) + `git ls-files --others --exclude-standard` cho file mới chưa track.
- `erp-cluvix` — nơi code BE nằm ⇒ **không** sinh delta, nhưng khi lần sau thấy hàng loạt
  path chết thì so 2 SHA này ra đúng đợt restructure nào đã gây ra.

**Mốc tính bằng `git diff`, KHÔNG bằng nhật ký tự khai.** Ledger phụ thuộc phiên trước có
ghi trung thực hay không — mà chính vấn đề từng phải dọn (90% path lệch, 18 file persona
tưởng drift) là bằng chứng kỷ luật ghi chép không đủ tin. `git diff` là dấu vết không cần
ai nhớ ghi.

## Chốt mốc là ĐIỀU KIỆN CHẶN, không phải bước dọn dẹp

`--stamp` **từ chối chạy** khi còn gate đỏ. Lý do: chốt mốc lúc còn việc chưa xử thì lần
sau delta bắt đầu từ mốc mới và **bỏ qua vĩnh viễn** phần dở — im lặng, không có gì báo ra.
Chạy dở giữa chừng thì thà không chốt, lần sau soi rộng hơn một chút còn hơn mất hẳn.

SHA không tồn tại (rebase, clone lại) → script tự hạ về `full` kèm cảnh báo, **không đoán**.

## 7 gate

| Gate | Bắt gì |
|---|---|
| `verify-paths` | Đường dẫn CODE trong knowledge có tồn tại thật |
| `verify-links` | Link giữa các file knowledge có trỏ file có thật |
| `verify-symbols` | **Hàm/type skill nhắc có TỒN TẠI trong code không** — gate duy nhất trả lời "skill khớp code", không chỉ "link còn sống" |
| `verify-frontmatter` | `name` khớp thư mục · chỉ `name`+`description` · description có "khi nào" |
| `verify-depth` | Body <500 dòng · reference phẳng 1 tầng · >100 dòng phải có mục lục |
| `verify-no-dup` | File trùng nội dung mà không phải symlink — **chỉ trong CÙNG 1 repo** |
| `verify-orphan` | File knowledge có ĐƯỜNG ĐI tới từ cửa vào không — chiều ngược của `verify-links` (nó kiểm link đi ra, gate này kiểm ai trỏ vào). Ngoại lệ: `_tooling/verify-orphan-allow.txt` |

Mỗi gate có `--self-test` chạy TRƯỚC khi quét. Gate xanh mà không tự chứng minh được thì
vô nghĩa — nó có thể xanh vì pattern không khớp gì cả.

⚠ **Quote glob khi grep**: `--include='*.md'`. Viết trần thì zsh nuốt glob → 0 hit → lưới
an toàn LUÔN xanh kể cả khi còn lỗi thật.

## Bài học đã trả giá

- **Gate kiểm đường dẫn KHÔNG đủ để kết luận skill khớp code.** `verify-paths` chỉ hỏi "thư mục có tồn tại"; nó cho qua ca doc ZNS mô tả 2 hàm gửi (tên + chữ ký đầy đủ) mà hàm **chưa bao giờ tồn tại** trong code. Đó là lý do có `verify-symbols`.
- **Phạm vi quét sai làm gate báo động giả hàng loạt.** `verify-symbols` bản đầu bỏ sót `mobile/packages/` → `MasterDataSync` bị báo "không tồn tại" oan dù nó nằm ở `core_domain/lib/src/di.dart`. Thêm gốc code mới vào repo thì phải thêm vào `ROOTS` của gate.
- **Gate sai còn nguy hiểm hơn không có gate.** `verify-no-dup` từng bắt 2 file command
  trùng giữa `frontend/` và `mobile/` — nhưng đó là 2 repo độc lập phải chạy standalone,
  symlink sang repo cha sẽ gãy khi clone riêng. Suýt "dọn" theo gate và phá thứ đang đúng.
  Luật "1 nguồn + symlink" **chỉ khả thi trong phạm vi 1 repo**.
- **Sửa link hàng loạt theo basename là bẫy.** Bản đồ `INDEX.md → <đích>` khiến MỌI tham
  chiếu `INDEX.md` chết bị đổi thành cùng một đích sai, lan ~26 chỗ. Sửa hàng loạt phải
  khoá theo **đường dẫn đầy đủ**, không theo tên file.
- **Delta so với `HEAD` bỏ lọt đúng phần mới nhất.** `/save-skill` ghi xong thường KHÔNG commit
  ngay (và nhiều phiên chạy song song thì càng lâu mới commit). Bản đầu so `<mốc>..HEAD` ⇒ 322
  dòng knowledge vừa ghi trong ngày, gồm **6 file hoàn toàn mới**, vô hình với gate delta —
  chốt mốc xong vẫn báo sạch. Đã sửa: so với working tree + liệt file untracked. Ca đã lọt
  (2026-09-07): `writing-mysql-migrations` §tiền-tố-trái nói "bỏ được" trong khi
  `perf-list-endpoints` đo được bỏ `(company_id)` làm phân trang sâu 869ms → 4.793ms — **hai
  phiên ghi hai chiều ngược nhau, không phiên nào thấy phiên kia**. Mâu thuẫn nguy hiểm hơn
  trùng lặp: trùng thì thừa, mâu thuẫn thì người đọc làm hỏng thật.
- **Chạy nhiều `/save-skill` song song ⇒ soi CHÉO theo chủ đề, không chỉ theo file.** Hai phiên
  ghi về cùng một chủ đề vào hai file khác nhau thì dedup theo file không thấy gì. Cách rẻ:
  lấy 5-7 từ khoá đặc trưng của đợt vừa ghi, grep TOÀN corpus, đọc từng cụm cùng chủ đề xem
  chúng bổ sung hay cãi nhau.
- **Kiểm tra sau khi hành động thì không phải kiểm tra.** Nối `git add` + `git diff --cached`
  + `git commit` trong một lệnh ⇒ output kiểm tra về tới nơi thì commit đã chạy. Tách 2 lượt.
- **Quy tắc theo tên file không đáng tin bằng kiểm nội dung.** Luật "file `INDEX.md` là điều
  hướng, không mang theo" đã suýt làm mất 6 khối nội dung độc nhất (hằng số nghiệp vụ, bảng
  enum, bảng giải thích tách package). Luôn grep xác minh trước khi bỏ.

## Đo hiệu quả skill (khi cần)

Phương pháp: giao task thật cho agent **không** có skill → ghi hành vi; giao lại **có**
skill → so. Nguồn task: case đã từng làm sai trong `bmad/done/`.

⚠ **Giới hạn đã đo (2026-08-17)**: không tạo được baseline sạch bằng Agent tool — project
context (CLAUDE.md, MEMORY.md của phiên) tiêm ở cấp session qua `system-reminder`, `cwd` và chỉ
dẫn văn bản đều không chặn được. Nhiễm bẩn lệch kết quả về phía "không khác biệt", nên:
- Có khác biệt = **bằng chứng dương, thận trọng** (baseline có thêm thông tin mà vẫn sai).
- Không khác biệt = **không kết luận được**, KHÔNG phải bằng chứng skill vô dụng.

Và giá trị skill **không đồng đều theo loại knowledge**: rule/contract/registry không hiện
trong code thì chặn sai thật; bug đã fix + đã có comment tại chỗ thì model mạnh tự suy lại
được, skill chỉ giúp nhanh và đúng con số cụ thể.
