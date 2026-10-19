# UI palette & component reference

## Mục lục

- Primary Color (Single Brand Hero)
- Color Roles
  - Primary — Purple `#7B3F9E`
  - Accent — Orange `#FF5722`
  - Danger — Red `#E53935`
  - Gradient Partner — Burgundy `#8B1538`
- Neutral Colors
- Semantic (System) Colors
- Gradient
- Chart Colors (ECharts)
  - Rule dựng chart (nguồn: `nextlevelbuilder/ui-ux-pro-max-skill`, MIT)
- Z-index scale
- Component Quick Reference
  - Buttons
  - Text
  - Cards
  - Report list panel (menu + mô tả)
  - Cờ quốc gia
- Accessibility
  - Bảng contrast đã ĐO — palette hiện tại trên nền trắng
- Responsive — 5 khung màn hình BẮT BUỘC kiểm
- Don'ts

---


## Primary Color (Single Brand Hero)

**Purple `#7B3F9E`** — màu chủ đạo duy nhất của Cluvix.

Mọi element quan trọng trong UI đều dùng Purple. Các màu còn lại phục vụ Purple, không cạnh tranh ngang hàng.

---

## Color Roles

### Primary — Purple `#7B3F9E`
Dùng cho:
- Primary buttons
- Active states, selected states
- Navigation highlight
- Links
- Important icons
- Headers, labels quan trọng
- Focus ring (input focus)

Variants:
- Light: `#9B5FBE` — hover states nhẹ, background tint
- Dark: `#6A3589` — pressed states, hover trên primary button

### Accent — Orange `#FF5722`
Dùng cho:
- Badges, notification counters
- Số liệu nổi bật (revenue, KPI quan trọng)
- Highlights cần kéo mắt người dùng
- Tags đặc biệt

Không dùng: buttons, navigation, headings.

### Danger — Red `#E53935`
Dùng cho:
- Error messages, validation errors
- Destructive action buttons (Delete, Cancel)
- Critical warnings
- Border/label khi input invalid

Không dùng như brand color — chỉ xuất hiện khi có vấn đề cần xử lý.

### Gradient Partner — Burgundy `#8B1538`
Không dùng độc lập trong UI components.

Chỉ dùng trong:
- Gradient logo / hero banner: `from-[#7B3F9E] to-[#8B1538]`
- Background decorative (opacity thấp, làm nền)

---

## Neutral Colors

> ⚠ **KHÔNG hardcode các hex/class gray này cho surface & text.** Đây là **giá trị mà token resolve ra ở light mode**, ghi lại để tham chiếu — KHÔNG phải class được phép viết thẳng vào template. Trong code PHẢI dùng token theo bảng `frontend/skill/skills/frontend-hard-rules/SKILL.md` §B.18 (`bg-[var(--p-content-background)]`, `text-[var(--p-text-color)]`, `text-[var(--p-text-muted-color)]`, `border-[var(--p-content-border-color)]`), nếu không dark mode sẽ vỡ. Chỉ màu brand/semantic/chart mới giữ hex trực tiếp.

| Gray | Hex (light) | Vai trò | Token phải dùng trong code |
|---|---|---|---|
| 50 | `#F9FAFB` | page background | `bg-[var(--p-content-background)]` (hoặc nền layout sẵn có) |
| 100 | `#F3F4F6` | card background, divider | `bg-[var(--p-content-background)]` / `border-[var(--p-content-border-color)]` |
| 200 | `#E5E7EB` | border default | `border-[var(--p-content-border-color)]` |
| 300 | `#D1D5DB` | placeholder, disabled border | token PrimeNG của control, không tự set |
| 500 | `#6B7280` | muted text | `text-[var(--p-text-muted-color)]` |
| 600 | `#4B5563` | body text | `text-[var(--p-text-color)]` |
| 700 | `#374151` | secondary heading | `text-[var(--p-text-color)]` |
| 800 | `#1F2937` | primary heading | `text-[var(--p-text-color)]` |
| 900 | `#111827` | emphasis text | `text-[var(--p-text-color)]` |

---

## Semantic (System) Colors

Các màu này không phải brand — chỉ dùng cho trạng thái hệ thống:

- Success: `#10B981` (Green)
- Warning: `#F59E0B` (Amber) — **chỉ dùng cho NỀN/VIỀN** (thường ở dạng alpha, vd `bg-[#F59E0B14]` + `border-[#F59E0B80]`)
- Warning on-surface: `#B45309` (Amber đậm) — dùng cho **CHỮ/ICON** cảnh báo trên nền sáng
- Error: `#E53935` (Red — trùng Danger)
- Info: `#3B82F6` (Blue)

> **Vì sao warning có 2 giá trị** (user chốt 2026-08-17): `#F59E0B` chỉ đạt **2.15:1** trên nền trắng
> — trượt cả ngưỡng 3:1 của non-text, nên không được dùng làm màu chữ/icon. `#B45309` đạt **5.02:1**,
> qua AA text thường. Đây là giá trị đã tính sẵn ở bảng "phương án thay thế" cuối file, nay ĐƯỢC DUYỆT
> cho vai trò on-surface. **Phạm vi duyệt chỉ có vậy**: 5 màu ❌ còn lại (`#EF4444`, `#E53935`, Gray 400,
> chart Green/Amber) VẪN giữ nguyên, không tự đổi. Code cũ đang dùng `#F59E0B` làm màu chữ thì đổi dần
> khi có việc chạm vào, không mở sweep riêng.
> Đã áp: `omnichat/sidebar/omnichat-sidebar.html` (khối cảnh báo "tạo khách xong nhưng chưa gắn được").

---

## Gradient

- Brand: `from-[#7B3F9E] to-[#8B1538]` (logo, hero banner)
- Primary tint: `from-[#7B3F9E]/10 to-[#7B3F9E]/5` (background card)

---

## Chart Colors (ECharts)

- Series 1: `#7B3F9E` (Purple — main)
- Series 2: `#FF5722` (Orange — secondary)
- Series 3: `#10B981` (Green — tertiary)
- Series 4: `#3B82F6` (Blue — quaternary)
- Series 5: `#F59E0B` (Amber — quinary)

### Rule dựng chart (nguồn: `nextlevelbuilder/ui-ux-pro-max-skill`, MIT)

- **Empty state**: không có dữ liệu → hiện thông điệp rõ ("Chưa có dữ liệu" + gợi ý), KHÔNG render khung trục trống trơn (user tưởng lỗi tải).
- **Loading**: skeleton/shimmer đúng kích thước chart, KHÔNG hiện khung trục rỗng rồi mới nhảy số (gây CLS).
- **Lỗi tải**: hiện message + nút thử lại, không để chart vỡ/trắng câm lặng.
- **Pie/donut tối đa 5 nhóm** — nhiều hơn thì chuyển bar; mắt người không so sánh được góc quạt khi có >5 lát.
- **Gridline nhạt** (gray-200 hoặc `var(--p-content-border-color)`) — không để tranh chấp thị giác với đường dữ liệu.
- **Contrast dữ liệu**: line/bar so với nền ≥3:1; text nhãn ≥4.5:1. ⚠ Series 3 (Green `#10B981` = 2.54) và Series 5 (Amber `#F59E0B` = 2.15) **hiện KHÔNG đạt** — xem bảng contrast cuối file. Chart dùng 2 series này bắt buộc có nhãn/legend trực tiếp, không để người đọc phải phân biệt bằng màu.
- **Không dùng màu làm tín hiệu duy nhất**: 5 màu series ở trên có cặp Green/Orange gần nhau — người mù màu đỏ-lục không phân biệt được. Bắt buộc kèm legend/nhãn trực tiếp, cần thì thêm pattern/dash khác nhau.
- **Legend luôn hiện**, đặt sát chart (không đẩy xuống dưới fold phải cuộn mới thấy).
- **Số/ngày trên trục và tooltip** đi qua `Intl.NumberFormat`/`Intl.DateTimeFormat` theo locale — không tự ghép chuỗi (xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.19).
- **Chart không phải là dữ liệu duy nhất**: màn hình báo cáo nên có bảng số đi kèm (hoặc export) — screen reader không đọc được canvas ECharts.

---

## Z-index scale

Thang cố định, KHÔNG chế số mới giữa các bậc (đang có `z-50` rải rác thiếu quy ước):

| Bậc | Dùng cho |
|---|---|
| `z-0` | Nội dung thường |
| `z-10` | Element nổi trong 1 card (badge góc, nút overlay ảnh) |
| `z-20` | Sticky header trong bảng/panel |
| `z-50` | Sticky footer dialog (`app-dialog-footer`), sticky toolbar |
| PrimeNG tự quản | Dialog / overlay / tooltip / toast — KHÔNG tự set z-index đè lên, chỉnh qua `primeng` z-index config nếu buộc phải |

Tự nâng z-index để "cho nó nổi lên" mà không theo thang là cách tạo bug chồng lớp về sau — kiểm tra bậc nào đang thiếu trước khi tăng số.

---

## Component Quick Reference

### Buttons
- Primary: `bg-[#7B3F9E] hover:bg-[#6A3589]`
- Secondary: outlined với border/text purple
- Danger: `bg-[#E53935] hover:bg-[#D32F2F]`

### Text
> Hex dưới đây là giá trị light-mode để tham chiếu — trong code dùng token B.18, KHÔNG viết `text-gray-*`.

- Heading: Gray 800 `#1F2937` → `text-[var(--p-text-color)]`
- Body: Gray 600 `#4B5563` → `text-[var(--p-text-color)]`
- Muted: Gray 500 `#6B7280` → `text-[var(--p-text-muted-color)]`
- Link / brand: Purple `#7B3F9E` (giữ hex — màu brand)
- ⚠ **KHÔNG dùng Gray 400 `#9CA3AF` / `text-surface-400` cho text đọc được** — chỉ 2.54:1, trượt AA rõ rệt (xem bảng contrast cuối file). Hint/description dùng Gray 500 (`text-[var(--p-text-muted-color)]`).
- Font-size lặp lại ≥ 3 lần trong 1 template qua inline `style="font-size: ..."` → định nghĩa 1 class dùng chung trong `:host` SCSS (vd `.cx-rail-ic { font-size: 0.846rem }`), KHÔNG lặp lại inline mỗi phần tử. 1-2 chỗ dùng duy nhất thì inline vẫn OK.

### Cards
> Hex tham chiếu light-mode — trong code dùng token, KHÔNG `bg-white`/`border-gray-*`.

- Background: White `#FFFFFF` → `bg-[var(--p-content-background)]`
- Border: Gray 100 `#F3F4F6` → `border-[var(--p-content-border-color)]`
- Hover border: Gray 200 `#E5E7EB` → `hover:border-[var(--p-content-border-color)]` / `bg-[var(--p-content-hover-background)]`

### Report list panel (menu + mô tả)
- Header: `bg-gradient-to-r from-[#7B3F9E]/12 to-[#7B3F9E]/5` — KHÔNG dùng `bg-gray-50` (quá nhạt)
- Title item: lấy theo `iconClass` của icon đó (đồng bộ icon+text), KHÔNG dùng `text-gray-600` cố định
- Description hiển thị **luôn** dưới title (title + `text-xs text-gray-400`), KHÔNG dùng tooltip/hover-only
- Active/hover: chỉ đổi background (`bg-[#7B3F9E]/8` + `font-semibold`), KHÔNG đổi màu text (đụng màu iconClass)
- Ref: `AccountantReportPanel`, `CrmReportPanel`

### Cờ quốc gia
- Dùng sprite có sẵn `flags.css` (global import `styles.scss`), KHÔNG tự chế emoji/SVG mới:
```html
<img [class]="'flag flag-' + countryCode" src="https://primefaces.org/cdn/primeng/images/demo/flag/flag_placeholder.png" [alt]="countryCode" />
```
- `countryCode` lấy từ phần sau dấu `-` của locale, lowercase (vd `en-US` → `us`)
- Ref: `sale/opportunity/portrait/status.html`, `modules/utilities/switch-language/`, `modules/auth/login/login.ts`

---

## Accessibility

- Contrast ratio tối thiểu 4.5:1 cho text thường; **3:1** cho text lớn (≥18.66px bold / ≥24px) và cho non-text UI (viền input, icon mang nghĩa, ranh giới control)
- Không dùng màu đơn độc để truyền trạng thái — luôn kèm icon
- **Dark mode phải đo riêng** — không suy ra từ giá trị light mode; cùng 1 cặp màu có thể đạt ở theme này và trượt ở theme kia
- **Đo tương phản phải tính `opacity`** — `getComputedStyle(el).color` trả màu TRƯỚC khi nhân opacity, nên đo trực tiếp từ nó cho số SAI ở mọi phần tử có `opacity < 1` (kể cả opacity thừa kế từ tổ tiên). Phải nhân dồn opacity từ phần tử lên tới nền rồi trộn: `eff = a·fg + (1−a)·bg`. Đo hụt 2026-09-04: kết luận "mọi chữ đều trắng 6.87:1" trong khi thực tế `opacity:.85` cho `#E2E5F4` = 5.48:1. Kết luận contrast mâu thuẫn với ảnh chụp → tin ảnh chụp, đo lại.
- **Đo trên nền THẬT nơi phần tử tiếp giáp** — phần tử chỉ hiện ở trạng thái active thì nền là **composite tint** (`bg-highlight` chồng nền khối), không phải nền khối trần: sai nền cho 3.06:1 trong khi thật là 2.61:1. Ở dark, tăng tương phản phải chọn bậc **nhạt hơn** (`primary.300` = 3.68:1); bậc đậm hơn (`primary.600` = 1.64:1) làm giảm.

### Bảng contrast đã ĐO — palette hiện tại trên nền trắng

Đo bằng công thức WCAG 2.x (relative luminance), 2026-08-16. **Đừng đoán bằng mắt — tra bảng này.**

| Màu | Hex | Tỷ lệ | Kết luận |
|---|---|---:|---|
| Heading Gray 800 | `#1F2937` | 14.68 | ✅ AA text thường |
| Burgundy | `#8B1538` | 9.28 | ✅ AA text thường |
| Purple dark | `#6A3589` | 8.46 | ✅ AA text thường |
| Body Gray 600 | `#4B5563` | 7.56 | ✅ AA text thường |
| **Purple primary** | `#7B3F9E` | 6.89 | ✅ AA text thường |
| Muted Gray 500 | `#6B7280` | 4.83 | ✅ AA (còn 4.63 trên nền Gray 50 — vẫn đạt) |
| Purple light | `#9B5FBE` | 4.40 | ⚠️ Trượt AA — chỉ dùng làm nền/tint, KHÔNG làm màu chữ |
| Danger Red | `#E53935` | 4.23 | ❌ Trượt AA cho text thường |
| `text-red-500` (đang dùng cho validation) | `#EF4444` | 3.76 | ❌ Trượt AA cho text thường |
| Warning on-surface | `#B45309` | 5.02 | ✅ AA text thường — màu chữ/icon cảnh báo (duyệt 2026-08-17) |
| Accent Orange | `#FF5722` | 3.16 | ⚠️ Chỉ đạt cho text LỚN / non-text |
| Warning Amber | `#F59E0B` | 2.15 | ❌ Chỉ làm nền/viền (alpha) — KHÔNG dùng cho chữ/icon |
| **Gray 400** | `#9CA3AF` | 2.54 | ❌ Trượt rõ — KHÔNG dùng cho text |

Chart series (yêu cầu ≥3:1 cho vùng dữ liệu):

| Series | Hex | Tỷ lệ | Kết luận |
|---|---|---:|---|
| S1 Purple | `#7B3F9E` | 6.89 | ✅ |
| S4 Blue | `#3B82F6` | 3.68 | ✅ |
| S2 Orange | `#FF5722` | 3.16 | ✅ (sát ngưỡng) |
| S3 Green | `#10B981` | 2.54 | ❌ Trượt 3:1 |
| S5 Amber | `#F59E0B` | 2.15 | ❌ Trượt 3:1 |

**Nợ chưa xử (user đã biết, chờ quyết định — KHÔNG tự đổi màu):** 5 màu ❌ ở trên đang dùng thật trong code.
Phương án thay thế đã tính sẵn nếu sau này chốt đổi: `#EF4444`→`#DC2626` (4.83) · `#E53935`→`#C62828` (5.62) · Gray 400→Gray 500 · chart Green `#10B981`→`#059669` (3.77) · Amber `#F59E0B`→`#B45309` (5.02).
Trong lúc chưa đổi: các màu trượt PHẢI có tín hiệu bù — icon kèm text lỗi (không chỉ màu đỏ), legend/nhãn trực tiếp cho chart (không bắt phân biệt bằng màu).

**Nợ sweep `text-*-400` (đo 2026-08-16):** còn **494 chỗ / 193 file** dùng `text-surface-400`/`text-gray-400` (246 trong `.html`, 248 trong template inline `.ts`; ~37 chỗ nằm trên icon `pi`). Cả 2 class đều KHÔNG theo theme và chỉ đạt 2.54:1.
CỐ Ý chưa sweep hàng loạt: cần xét từng ca (chữ đọc được → phải đổi `text-muted-color`; icon trang trí / đường phân cách → chấp nhận được), và 193 file là quá rộng để sửa cơ học an toàn khi có phiên khác đang sửa cùng repo. Quy ước từ nay: **code MỚI dùng `text-muted-color`**; file cũ đổi dần khi có việc chạm vào, không mở sweep riêng.
Đã xử trong đợt này (component dùng chung, blast radius lớn): `share/components/dialog-footer.ts` (`bg-white`→token), `share/components/customer-name.ts` (2 icon), `share/components/tooth-new/tooth-new.html` (`hover:bg-gray-50`→`hover:bg-emphasis`). **KHÔNG đổi** `share/components/print-signature-block.ts` — component in ra giấy, màu cố định là đúng.
Copy markup từ template tham khảo ngoài (ultima, mockup.html...) → PHẢI `grep -rn "text-surface-\|bg-surface-\|border-surface-[0-9]"` ngay trên các file vừa tạo TRƯỚC khi coi là xong — template ngoài không theo convention token của repo nên lỗi này lặp lại rất dễ (2 lần liên tiếp trong cùng 1 slug, `teamchat-menu.html` rồi `teamchat-page.html`).
- Vùng bấm tối thiểu **24×24 CSS px** (WCAG 2.2 AA) — đây là ngưỡng web, KHÔNG phải 44pt của mobile native
- Rule đầy đủ (focus không bị che, wrap URL dài, aria-sort, reduced-motion...): `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.19 + B.19b

---

## Responsive — 5 khung màn hình BẮT BUỘC kiểm

**Mọi việc đụng UX/UI (dựng mới, sửa layout, đổi spacing/kích thước, thêm cột/panel, animation
đóng-mở) PHẢI được kiểm ở đủ 5 khung dưới đây trước khi báo xong** — không chỉ nhìn khung máy đang
mở. Danh sách bám đúng `screens` trong `frontend/tailwind.config.js` (`sm 576 / md 768 / lg 992 /
xl 1200 / 2xl 1920`), phủ từ khung NHỎ NHẤT tới khung LỚN NHẤT mà người dùng thật đang dùng.

| # | Bề rộng | Đại diện | Điều dễ vỡ nhất ở khung này |
|---|---|---|---|
| 1 | **375px** | mobile nhỏ nhất còn hỗ trợ (iPhone SE/mini) | tràn ngang, bảng/chart không cuộn được, nút bị đẩy ra ngoài, dialog cao quá màn |
| 2 | **768px** | tablet dọc (`md`) | grid 2 cột ép thành 1, sidebar/drawer, toolbar filter xuống dòng |
| 3 | **992px** | biên `lg` — ranh giới drawer ↔ cột grid | dải "chết" khi CSS và TS lệch ngưỡng: panel vừa là cột vừa bị ép overlay |
| 4 | **1440px** | laptop phổ biến nhất (MacBook/FHD scale) | khung làm việc thật của hầu hết nhân viên — mọi mật độ/spacing chốt ở đây |
| 5 | **1920px** | màn hình lớn nhất (`2xl`, FHD desktop) | content dàn mỏng, cột số trôi xa nhãn, khoảng trắng chết giữa trang |

**Cách kiểm:**
- Trình duyệt: DevTools responsive mode. ⚠ Chrome MCP `resize_window` **KHÔNG** đổi được viewport render (báo thành công nhưng `innerWidth` giữ nguyên; `window.open` popup bị chặn vì không có user gesture) ⇒ agent không tự kiểm được 5 khung. Kiểm nội dung rule bằng cách bật `@media` qua CSSOM (`rule.media.mediaText='all'`, nhớ trả lại) — đúng rule sản phẩm, nhưng KHÔNG thay được viewport thật; luôn giao 5 mốc cho user thử tay.
- ⚠ Toạ độ click/hover của Chrome MCP (`computer`) tính theo **pixel ẢNH CHỤP**, KHÔNG phải CSS px. Quy đổi: `x_ảnh = x_css × (rộng_ảnh / innerWidth)` (đo 2026-09-04: ảnh 1512 vs `innerWidth` 1397 ⇒ lệch ~8%). Bỏ bước này thì click trượt các đích nhỏ (nút ≤24px) trong khi đích lớn vẫn trúng ⇒ rất dễ kết luận nhầm "chức năng hỏng". Trước khi báo một nút không chạy: kiểm `document.elementFromPoint(x_css, y_css)` xem đích có đúng ở đó không.
- Layout có breakpoint riêng (grid nhiều tier như omnichat) → kiểm THÊM đúng 2 mép của mỗi
  `@media`: `giá trị` và `giá trị − 1px`. Lỗi responsive tốn kém nhất luôn nằm ở mép, không nằm ở
  giữa dải: một bên CSS coi là desktop, một bên TS coi là mobile.
- Đọc số breakpoint từ `tailwind.config.js` + hằng số trong TS của chính component (vd
  `OMNI_DESKTOP_MIN_WIDTH`) — **CSS và TS lệch nhau 1px là sinh dải viewport hỏng**, đã xảy ra thật.
- Không tự dựng được môi trường chạy (agent không được start dev server) → nói rõ "chưa kiểm 5
  khung" và liệt kê đúng 5 mốc cho user thử tay, KHÔNG im lặng bỏ qua.

---

## Don'ts

- ❌ Không dùng Burgundy độc lập trong UI components
- ❌ Không dùng 2+ brand colors song song trong cùng 1 component
- ❌ Không dùng Orange/Red cho navigation hoặc headings
- ❌ Không dùng pure black `#000000`
- ❌ Không tạo màu brand mới ngoài danh sách này
