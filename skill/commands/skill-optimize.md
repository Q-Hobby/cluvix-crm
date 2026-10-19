Tối ưu knowledge base `skill/` — chạy lại được, chỉ soi phần MỚI kể từ lần trước.

**Đây là vỏ mỏng.** Logic nằm ở `skill/skills/_tooling/optimize.sh` (bash thuần) — cố ý, vì knowledge base phục vụ nhiều agent (Claude, Codex, Gemini) và cả người; đặt logic trong slash command là khoá năng lực bảo trì vào một runtime.

---

## Bước 1 — Chạy script

```bash
cd /Users/KemBi/www/erp-cluvix
skill/skills/_tooling/optimize.sh          # mặc định: tăng dần
skill/skills/_tooling/optimize.sh --full   # bỏ qua mốc (sau restructure lớn, hoặc nghi mốc sai)
```

Exit 0 = không còn việc. Exit 1 = có việc. Exit 2 = lỗi cấu hình.

## Bước 2 — Xử phần script chỉ ra

**Gate đỏ (tầng FULL)** → sửa trước, đây là lỗi do môi trường trôi (code đổi chỗ, doc đứng yên và tự thành sai).

**Delta có file** → chỉ soi ĐÚNG những file đó, KHÔNG soi lại toàn corpus:
1. Xếp chỗ theo bảng phân loại trong `skill/HOW_TO_UPDATE.md`.
2. **Dedup đối chiếu TOÀN corpus** — không chỉ so delta với delta. Trùng lặp điển hình là rule mới ghi lại đúng cái đã nằm trong file 6 tháng chưa ai đụng.
3. Body SKILL.md vượt 500 dòng → tách xuống `reference/`.
4. `description` còn nêu đủ "làm gì + KHI NÀO" sau khi thêm nội dung không.
5. Test lại fresh-context: skill có file trong delta + skill từng đánh dấu `weak` + ngẫu nhiên ≥3 skill khác.

## Bước 3 — Chốt mốc

```bash
skill/skills/_tooling/optimize.sh --stamp
```

Script **TỪ CHỐI chốt** khi còn gate đỏ. Đừng tìm cách lách: chốt mốc lúc còn việc chưa xử thì lần sau delta bắt đầu từ mốc mới và **bỏ qua vĩnh viễn** phần dở, im lặng không báo gì.

---

## Khi nào chạy

- Sau một đợt BMAD lớn (nhiều story ghi knowledge).
- Sau restructure module / đổi tên package / squash migration — dùng `--full`.
- Định kỳ, khi thấy `skill/` đã lâu không rà.

## Chi tiết cơ chế

`skill/skills/maintaining-skill-base/SKILL.md` — hai tầng FULL/delta và vì sao không gộp được, cách mốc hoạt động, 5 gate, và các bài học đã trả giá (gate sai còn nguy hiểm hơn không có gate; sửa link hàng loạt theo basename là bẫy).
