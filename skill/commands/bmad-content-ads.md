Chạy persona Content Writer (BMAD) — sinh trực tiếp bộ content quảng cáo Facebook Ads (headline, primary text, CTA, biến thể A/B) từ 1 yêu cầu. KHÔNG đi qua vòng đời 6 giai đoạn brief→prd→architect→stories→dev→qa — đây là command đơn, 1 lượt.

---

## Bước 0 — Xác định slug

1. Có tham số truyền vào (`$ARGUMENTS`) → dùng làm gợi ý slug chiến dịch (kebab-case, ngắn gọn).
2. Không có tham số → hỏi user tên sản phẩm/chiến dịch để đặt slug.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/content-writer.md` — làm đúng theo persona đó (input/output/nguyên tắc).

---

## Bước 2 — Thu thập input

Nếu user chưa cung cấp đủ trong yêu cầu, hỏi (KHÔNG tự đoán rồi viết):
1. Sản phẩm/dịch vụ + điểm khác biệt.
2. Mục tiêu chiến dịch (nhận diện / lead / chuyển đổi / retarget).
3. Đối tượng mục tiêu.
4. Tone giọng văn.
5. Nền tảng (mặc định Facebook Ads nếu không nói khác).

Nếu quảng cáo dịch vụ y tế/thẩm mỹ (phòng khám, nha khoa, làm đẹp) → cảnh báo user về giới hạn quảng cáo y tế theo pháp luật VN trước khi viết (xem persona mục "Đọc trước khi viết").

---

## Bước 3 — Thực thi

1. Viết `documents/content-ads/<slug>/ad-copy.md` theo đúng cấu trúc trong `skill/bmad/personas/content-writer.md` — 3-5 biến thể A/B, mỗi biến thể một góc tiếp cận khác nhau.
2. KHÔNG bịa số liệu/cam kết — thiếu số liệu thật thì đánh dấu `[SỐ LIỆU CẦN XÁC MINH]`.

---

## Bước 4 — Báo cáo

- File đã tạo: `documents/content-ads/<slug>/ad-copy.md`
- Cảnh báo tuân thủ (nếu có) và giả định đã đưa ra khi thiếu input.
