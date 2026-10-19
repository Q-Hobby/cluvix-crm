Knowledge curator cho codebase Cluvix — trích xuất từ session và ghi vào `skill/` đúng chỗ, đúng format, KHÔNG trùng lặp.

---

## Bước 0 — Đọc trước

Đọc `skill/HOW_TO_UPDATE.md` — source of truth về cách ghi. Không làm gì trái với file đó.

---

## Bước 1 — Extract: Cái gì đáng lưu?

Lọc từ conversation hiện tại. Chỉ giữ những gì thỏa **cả 3 điều kiện**:

**A — Non-obvious:** không đọc code thì không biết · lần sau vẫn có thể mắc lại · là gotcha/edge case/quyết định kiến trúc không hiển nhiên.

**B — Tái sử dụng được:** áp dụng nhiều feature/module · là pattern/rule, không phải fix cho 1 bug cụ thể.

**C — Có gốc tích trong session:** chỉ ra được khoảnh khắc cụ thể sinh ra nó (bug vừa sửa, behavior gây bất ngờ, quyết định user vừa chốt). Không lưu "kiến thức chung" đúng ở mọi codebase (SOLID, nhớ unsubscribe, đặt tên rõ ràng...).

**Ràng buộc số lượng — chất lượng > số lượng:**
- Mặc định 1–3 mục/session, tối đa 5. Nhiều hơn → đang lưu cả impl detail, lọc lại.
- **1 mục = 1 fact.** Không gộp nhiều rule thành 1 mục dài; không kể chuyện quá trình.

**Loại bỏ:** chi tiết impl task vừa làm · thứ đọc code là hiểu · context lịch sử ("trước đây là X, giờ đổi thành Y" → chỉ ghi Y) · thứ git/CLAUDE.md đã ghi.

---

## Bước 2 — Dedup check TRƯỚC khi propose (bắt buộc từng mục)

```bash
grep -rn "<keyword>" skill frontend/skill mobile/skill --include="*.md"   # thử 2-3 keyword/biến thể mỗi mục
```

| Kết quả grep | Action |
|---|---|
| Đã có, cùng ý | **LOẠI** mục này — không đề xuất |
| Đã có một phần / sai / thiếu | Đổi thành **UPDATE** file hiện có (nêu rõ skill + section, chỉ ghi phần delta) |
| Chưa có | **ADD** — chọn skill đích theo bảng Step 1 của HOW_TO_UPDATE.md, ưu tiên skill hẹp nhất/đúng domain nhất (vd nghiệp vụ module cụ thể > hard-rules > cross-cutting) |

Mục nào chưa grep → chưa được đưa vào đề xuất.

---

## Bước 3 — Propose (KHÔNG ghi trước khi user duyệt)

```
## Đề xuất lưu vào skill/

### 1. [Tên ngắn gọn]
- Loại: rule / pattern / gotcha / business / config
- Gốc tích: [1 câu — từ đâu trong session]
- Dedup: 0 hit với "<keyword>" / đã có ở file X §Y → UPDATE phần delta
- File đích: skill/skills/<skill-name>/SKILL.md hoặc .../reference/<file>.md (frontend/skill/skills/... hoặc mobile/skill/skills/... nếu thuộc repo con)
- Action: UPDATE section "X" / ADD vào section "Y" / TẠO reference/ MỚI (phải thỏa 3 điều kiện của HOW_TO_UPDATE)
- Nội dung sẽ ghi (≤ 5 dòng + code block nếu cần):
  [bullet rule-trước, không "tại sao" trừ khi non-obvious, không narrative]
```

Hỏi: **"Bạn muốn lưu mục nào? (all / 1,3 / none)"**

---

## Bước 4 — Thực thi (sau khi user duyệt)

1. Ghi ĐÚNG nội dung đã duyệt ở Bước 3 — không phình thêm khi ghi
2. UPDATE → sửa in-place đúng section, diff tối thiểu, không format lại phần không liên quan
3. Tạo `reference/*.md` mới → đảm bảo SKILL.md của skill đó trỏ tới file mới (progressive disclosure)
4. Verify: grep lại keyword — chỉ xuất hiện **1 lần** trong toàn bộ knowledge base (`skill/` + `frontend/skill/` + `mobile/skill/`)
5. Verify: SKILL.md sau edit ≤ 500 dòng body — vượt thì tách sang `reference/` theo Step 4 HOW_TO_UPDATE; `reference/*.md` >100 dòng phải có mục lục

### Gate bắt buộc — chạy NGAY sau khi ghi, TRƯỚC khi báo Bước 5

```bash
cd /Users/KemBi/www/erp-cluvix
for s in verify-paths verify-links verify-symbols verify-frontmatter verify-depth verify-no-dup verify-orphan; do skill/skills/_tooling/$s.sh || echo "GATE FAIL: $s"; done
```

**TỪ CHỐI kết thúc nếu có `GATE FAIL` mới** — sửa ngay, không để lại cho session sau. Lý do: chặn lúc ghi (vài giây) rẻ hơn nhiều so với gom lại xử sau vài tháng.

⚠ Ngoại lệ đã biết: `verify-no-dup.sh` đỏ sẵn 1 nhóm (file `bmad-plan.md` của FE trùng của mobile) — nợ cũ chưa xử, **được phép giữ đỏ**. Bất kỳ finding nào KHÁC nhóm đó (kể cả từ `verify-no-dup.sh`) là finding MỚI → không được bỏ qua.

---

## Bước 5 — Báo cáo

```
## Đã lưu
| Mục | File | Action |
|-----|------|--------|

## Bỏ qua (kèm lý do: trùng / obvious / impl detail / user từ chối)
| Mục | Lý do |
|-----|-------|
```

---

## Cấm tuyệt đối

- Ghi trước khi user duyệt
- Đề xuất mục chưa grep dedup
- Narrative quá trình ("trong session này ta đã thử A rồi B...")
- Gộp nhiều fact vào 1 mục · mở section mới khi ghép được vào section có sẵn
- Ghi lại thứ chỉ đúng cho task vừa làm, không tái sử dụng được
