---
name: using-primeng-components
description: >-
  PrimeNG v22 (rc.2) + Angular 22 trong Cluvix: component quick reference, deprecated
  components giữ nguyên chủ động (p-multiSelect/p-password/p-galleria/p-colorPicker),
  brand palette, pricing tier option. Dùng skill này TRƯỚC KHI động vào các component
  trên, trước khi upgrade PrimeNG tiếp, hoặc khi làm feature FE mới cần biết API
  Angular 22 mới.
---

# Using PrimeNG components

> UI chi tiết (palette đo contrast, component reference, don'ts): `reference/ui.md`
> Gotchas Build/Tooling/Git (AOT, ng build, script khảo sát): `reference/fe-patterns-gotchas-build.md`



> Trạng thái (2026-07-24): **Đã upgrade lên `primeng@22.0.0` bản STABLE** (+ `@primeuix/themes@3.0.0`) — trước đó dùng `22.0.0-rc.2` từ lúc migrate (2026-07-09) vì khi đó v22 chỉ có RC. Đã diff source `onToggleAll`/`allSelected`/`isSelected` giữa rc.2 và 22.0.0 stable: **giống hệt nhau**, không có thay đổi hành vi — nâng cấp chỉ để thoát khỏi RC, KHÔNG tự fix bug MultiSelect select-all bên dưới. Build/tsc pass sau upgrade. `pTemplate` (129 file) đã migrate xong hoàn toàn. `p-multiSelect`/`p-password`/`p-galleria`/`p-colorPicker` — **PHẦN LỚN CHỦ ĐỘNG GIỮ NGUYÊN** (không migrate) do phát hiện feature-gap thật ở component thay thế, xem mục 2. Full log thực chiến `../bmad/primeng-v22-migration/` đã xóa khi dọn BMAD hoàn thành (2026-07-16) — gotchas đáng giữ đã đúc kết hết vào file này.
>
> **Bug upstream chưa fix — MultiSelect "chọn tất cả" + filter (2026-07-24):** repo `primefaces/primeng` trên GitHub đã **archived (read-only) từ 2026-06-28** — không còn nhận fix cộng đồng. Header "chọn tất cả" của `p-multiselect` (khi có `[filter]="true"`, dùng ở HẦU HẾT wrapper multiselect trong Cluvix) bấm BỎ chọn tất cả không clear đúng state — bug có thật, đã báo cáo nhiều lần trên GitHub, KHÔNG PHẢI lỗi code Cluvix: [#15516](https://github.com/primefaces/primeng/issues/15516), [#14603](https://github.com/primefaces/primeng/issues/14603), [#14252](https://github.com/primefaces/primeng/issues/14252) (fix cũ ở #14252 bị regress lại ở #15516, "Needs Triage", chưa fix ở bất kỳ version nào tính đến 22.0.0). Hướng xử lý (chưa làm, cần quyết định fan-out ~40 wrapper): tắt `[showToggleAll]="false"` + tự viết nút "chọn tất cả" trong `base-multiselect-control.ts` và các wrapper không dùng base, không phụ thuộc logic nội bộ PrimeNG nữa.

Nguồn: https://primeng.dev/migration/v22 (đọc 2026-07-08).

---

## 0. CRITICAL — v22 bắt buộc license key, kể cả bản miễn phí (KHÔNG có ở v21 trở về trước)

Từ v22, PrimeNG (rebrand "PrimeUI") verify license key offline qua `@primeui/license-manager` (chữ ký Ed25519), gọi qua `providePrimeNG({..., license: 'KEY'})`. Không có key hợp lệ → banner đỏ "Invalid PrimeUI License" góc dưới phải (không chặn thao tác, nhưng là vấn đề compliance thật).

- **Community License (miễn phí)**: doanh thu < $1M/năm, < 5 dev, < 10 nhân viên, < $3M vốn gọi được. Renew hàng năm.
- **Commercial License (trả phí)**: công ty không đạt điều kiện trên.
- Đăng ký: https://primeui.dev/licenses/community hoặc `/commercial`.
- **Hết hạn KHÔNG hỏng chức năng** — chỉ banner đỏ; `verified()` chỉ được đọc để hiện banner, không gate component nào.
- **Chỉ tier `community` bị kiểm theo đồng hồ** (`Date.now() > exp`, grace 30 ngày). Mọi tier còn gate thứ 2: `RELEASE_DATE` của bản PrimeNG > `exp` ⇒ "expired" NGAY, **không grace** — nâng PrimeNG lên bản phát hành sau ngày hết hạn key cũng cần key mới.

**Đây là quyết định kinh doanh, KHÔNG tự động hoá được** — phải chốt trước khi coi migration là hoàn tất, dù build/tsc/lint đều pass bình thường không có key.

**Cách lưu key (đổi 2026-09-05)**: key nạp lúc CHẠY từ file JSON trên CDN — `share/utils/primeui-license.util.ts`; `app.config.ts` đọc lazy `get license() { return primeuiLicense(); }` (gán thẳng sẽ đóng băng giá trị lúc load module). `environment.primeuiLicense` ở 3 file nay chỉ là FALLBACK. Gia hạn = sửa file trên CDN, KHÔNG build lại.

---

## 1. Breaking change quan trọng nhất — base font-size 14px → 16px

PrimeNG tính kích thước component theo `rem` dựa trên root font-size. v22 đổi mặc định từ 14px → 16px (khớp browser default). Có "compat variant" preset giữ 14px cho layout cũ.

**Trước khi upgrade:** xác nhận layout hiện tại (đặc biệt các bảng dày đặc, form nhiều field) không vỡ khi component to hơn ~14%. Nếu không muốn sửa layout ngay → bật compat variant thay vì để mặc định 16px.

---

## 2. Component deprecated (xoá hẳn ở v24) — kết quả migrate thật (2026-07-09)

**PHÁT HIỆN QUAN TRỌNG NHẤT của đợt migrate này: "deprecated, còn hoạt động tới v24" ≠ "thay thế mới có đầy đủ tính năng".** Ban đầu giả định thay component chỉ là đổi tên tag — THỰC TẾ hầu hết component thay thế ở generation v22 sớm này **thiếu tính năng thật so với bản cũ**, không phải chỉ khác API bề mặt. Trước khi migrate BẤT KỲ component nào trong bảng dưới, BẮT BUỘC so sánh 2 danh sách `@Input` qua `node_modules/primeng/types/primeng-{old}.d.ts` vs `primeng-{new}.d.ts` (lệnh: `grep 'ɵɵComponentDeclaration<X,' file.d.ts`), rồi grep xem file thật trong repo có dùng prop nào nằm trong phần "chỉ có ở bản cũ" không — nếu có, ĐỪNG migrate mù.

| Component cũ | Thay bằng | Số file thực tế | Kết quả |
|---|---|---|---|
| `pTemplate` | `ng-template` + reference variable | 129 | ✅ Migrate xong 100% — nhưng **KHÔNG tương đương**: xem cảnh báo ngay dưới bảng |
| `p-multiSelect` (`p-multiselect` sau rename casing) | `p-select` với `multiple` | 137 | ⚠️ **Migrate 54/137, giữ nguyên 83** — `p-select` thiếu `maxSelectedLabels`/`display`/`selectedItemsLabel`/`showToggleAll`, không có template hook thay thế. Chỉ migrate file KHÔNG dùng các prop này |
| `p-password` | directive `pPassword` | 2 | ❌ **Giữ nguyên cả 2** — directive KHÔNG có UI toggle tương tác (`showPassword` chỉ ép `type=text` 1 chiều, không phải nút bấm), KHÔNG có strength-meter overlay (`feedback` là dead prop trên directive) |

> ⛔ **`pTemplate` trong code MỚI = bảng rỗng, im lặng.** Bảng trên là nhật ký một đợt migrate đã xong,
> KHÔNG phải "chuyện đã đóng". Ở v22 `p-table` chỉ đọc `<ng-template #header|#body|#expandedrow|#emptymessage|#caption|#footer|#loadingbody>`;
> `pTemplate="header"` vẫn là directive HỢP LỆ nên `tsc` và `ngc --noEmit` đều xanh, nhưng component
> không nhận template ⇒ render ra bảng **không header, không dòng, không cả empty state**, không lỗi
> console. Dính thật 2026-09-09 trên production. Danh sách tên template + ví dụ ❌/✅:
> `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-form.md` §`pTemplate` deprecated.
>
> Trước khi viết một API PrimeNG theo trí nhớ, đếm xem repo đang dùng cách nào:
> `grep -rho '<ng-template #[a-z]*' --include='*.html' src/app | sort | uniq -c | sort -rn`.
> 440 file dùng `#header`, 1 file dùng `pTemplate` — file thiểu số là file sai.
| `p-galleria` | `p-gallery` | 1 | ❌ **Giữ nguyên** — `p-gallery` thiếu `value` (!), `numVisible`, `showThumbnails`, `circular`, `fullScreen`... không cách nào bind data được |
| `p-colorPicker` (`p-colorpicker` sau rename casing) | `p-inputColor` | 3 | ❌ **Giữ nguyên** — `p-inputcolor` là UI widget khác hẳn (0 overlay/panel logic, có vẻ dùng slider/area trực tiếp thay vì popup swatch cũ), đổi trải nghiệm quá lớn để migrate mù |
| `p-editor` | PrimeUI PRO Editor (trả phí) | 17 | ⛔ Out of scope — chờ quyết định licensing PrimeUI PRO |
| `p-chart` | PrimeUI PRO Charts (trả phí) | 13 | ⛔ Out of scope — chờ quyết định licensing PrimeUI PRO |
| `p-panelMenu` | `p-menu` với `toggleable` | 0 | — |
| `p-inputMask` | directive `pInputMask` | 0 | — |
| `p-scrollPanel` | `p-scrollArea` | 0 | — |
| `p-image` | `p-gallery` | 0 | — |

**Component nào "giữ nguyên" ở trên đều vẫn hoạt động bình thường** (chỉ deprecated, chưa xoá tới v24) — không phải lỗi, là quyết định có căn cứ. Revisit khi PrimeNG bổ sung tính năng tương đương cho bản thay thế, hoặc gần mốc v24 buộc phải xử lý.

**Input class có thể là input CHẾT — verify trước khi khoanh style qua nó:** đọc `classes.root` trong `node_modules/primeng/fesm2022/primeng-<comp>.mjs` xem input đó có được nối vào class thật không. `p-galleria` v22: `containerClass` khai nhưng KHÔNG gắn vào root (mọi CSS khoanh theo nó vô tác dụng); chỉ `maskClass` áp thật.

**`p-editor`/`p-chart` migrate sang PrimeUI PRO — cần xác nhận licensing trước khi upgrade**, không phải thuần code change.

**Nút con của `<p-buttongroup>` PHẢI giữ component `<p-button>`, KHÔNG đổi sang `<button pButton>`** — CSS buttongroup chọn theo tên thẻ (`.p-buttongroup p-button:not(:last-child) .p-button`), đổi sang directive thì các nút tách rời, bo tròn riêng lẻ; build không báo lỗi. Ngoài buttongroup vẫn dùng `<button pButton>`.

---

## 2.1 — p-tabs [lazy]="true" KHÔNG lazy thật (rc.2)

`TabPanel.shouldRender()` trả `true` cho tab không active ngay lần render đầu — panel vẫn ẩn đúng qua CSS
nhưng component con bên trong đã bị tạo, mọi HTTP fetch trong `ngOnInit` bắn ra ngay. Verified qua unit
test cô lập (TestBed + component thật từ node_modules/primeng), không phải do cách viết template của app.

✅ Fix: bỏ `[lazy]="true"`, bọc nội dung mỗi `<p-tabpanel>` (trừ tab mặc định active) bằng
`@defer (when activeTab() === 'X')` — tự có "render 1 lần rồi giữ" + code-split JS riêng từng tab.
Cần `activeTab` là signal + `[value]="activeTab()"` + `(valueChange)="activeTab.set(...)"` (controlled).

`p-tabpanel` ẩn panel bằng host attribute `hidden` → nội dung đã render GIỮ NGUYÊN trong DOM, `@defer`
chỉ chạy 1 lần (verified `node_modules/primeng/fesm2022/primeng-tabs.mjs`).

- `@defer` không khai `@placeholder` → render RỖNG cho tới khi `@loading` kịp hiện (`after 150ms`): panel sập về 0px rồi nhảy 2 nhịp. LUÔN khai `@placeholder`, và cho cả placeholder lẫn loading `min-height` bằng chiều cao panel.
- `min-height` đặt ở `<p-tabpanels>` (1 lần cho mọi tab), KHÔNG đặt riêng trong 1 panel — đặt lẻ thì card co giãn mỗi lần đổi tab.

---

## 3. Removed API (đã deprecated từ v20/v21, giờ xoá hẳn)

- `@primeng/themes` → `@primeuix/themes`
- `pTemplate` → `ng-template` + reference variable (xem bảng trên)
- **CamelCase selector → kebab-case/lowercase — ĐÂY LÀ ĐIỂM GÂY LỖI NHIỀU NHẤT KHI MIGRATE THẬT** (không phải chỉ vài attribute nhỏ như hiểu ban đầu — áp dụng cho TÊN COMPONENT SELECTOR). Component compile ra selector toàn lowercase-liền hoặc kebab-case (VD: `p-multiselect, p-multi-select`), selector camelCase cũ (`p-multiSelect`) KHÔNG còn match được nữa — Angular coi thẻ đó là unknown element, kéo theo TOÀN BỘ property binding trên thẻ báo lỗi "unknown property" (rất dễ nhầm tưởng property bị đổi/xoá, thực ra chỉ do thẻ không match). Danh sách đã xác nhận thật khi migrate Cluvix: `p-autoComplete`→`p-autocomplete`, `p-avatarGroup`→`p-avatargroup`, `p-buttonGroup`→`p-buttongroup`, `p-cascadeSelect`→`p-cascadeselect`, `p-cellEditor`→`p-celleditor`, `p-colorPicker`→`p-colorpicker`, `p-confirmDialog`→`p-confirmdialog`, `p-dataView`→`p-dataview`, `p-datePicker`→`p-datepicker`, `p-fileUpload`→`p-fileupload`, `p-inputGroup`→`p-inputgroup`, `p-inputGroupAddon`→`p-inputgroup-addon` (dạng đặc biệt, không phải kebab thuần), `p-inputNumber`→`p-inputnumber`, `p-multiSelect`→`p-multiselect`, `p-orderList`→`p-orderlist`, `p-progressBar`→`p-progressbar`, `p-progressSpinner`→`p-progressspinner`, `p-radioButton`→`p-radiobutton`, `p-selectButton`→`p-selectbutton`, `p-tableCheckbox`→`p-tablecheckbox`, `p-tableHeaderCheckbox`→`p-tableheadercheckbox`, `p-toggleSwitch`→`p-toggleswitch`. **Verify đúng bằng cách grep selector thật trong `node_modules/primeng/fesm2022/*.mjs`** (`selector: "..."`), KHÔNG đoán — vài selector không theo pattern kebab thuần (VD `inputGroupAddon`).
- `pButton` icon/label property → directive `pButtonIcon`/`pButtonLabel` (dùng content projection `<span pButtonLabel>{{...}}</span>`, không phải `[label]="..."` binding nữa)
- `p-message` mất input `[text]` — chuyển sang content projection `<p-message>{{...}}</p-message>`
- `p-drawer`/`p-dialog` mất `[transitionOptions]` — xoá binding (dùng animation mặc định) hoặc dùng `motionOptions` mới nếu cần tuỳ biến
- `p-tabs [scrollable]` deprecated (cuộn là mặc định) — bỏ attribute, không thì vỡ ở bản sau.
- `p-tabview`, `p-buttonset`, `p-fluid` không còn được emit ở v22 — CSS bám các class này là chết. Trước khi xoá CSS nghi chết: `grep -rl '<class>' node_modules/primeng/fesm2022/*.mjs`, rỗng mới xoá.
- `showTransitionOptions`/`hideTransitionOptions` → native CSS animation. Thay bằng override class `.{classname}-enter-active`/`.{classname}-leave-active` (định nghĩa animation name/duration/easing qua CSS, không qua property JS). 3 nhóm class theo component: anchored overlay (`p-anchored-overlay-*`, dùng cho Select/Popover/AutoComplete), collapsible (`p-collapsible-*`, dùng cho Accordion/Panel/Fieldset/Stepper), dialog/drawer (`p-dialog-*`/`p-drawer-*`). Override global (ảnh hưởng mọi component nhóm đó) hoặc scoped (gắn class riêng từng instance). Chi tiết: https://primeng.dev/guides/animations

---

## 4. InputMask — component → directive

`InputMask` chuyển từ component riêng sang directive `pInputMask` gắn lên `<input>` chuẩn (cùng pattern với `pInputPassword`, `pButtonIcon`). Repo hiện chưa dùng `p-inputMask` (0 file) — không phải audit khi upgrade, nhưng feature mới cần input mask thì viết thẳng theo API v22, không dùng API v21 cũ.

---

## 4.1 CommandMenu — component mới v22, KHÔNG có overlay built-in

`p-commandmenu`/`p-command-menu` chỉ là search-palette INLINE (nhận `model: MenuItem[]`, tự filter theo `search`, gọi `item.command` khi chọn) — không có `visible`/overlay/popup nào tự thân.
Muốn dùng kiểu Cmd+K command palette (bật/tắt qua phím tắt) → PHẢI tự bọc trong `p-dialog` (pattern giống `GlobalSearch`: 1 service `WritableSignal<boolean>` + `p-dialog [visible]="service.visible()"`), không có prop `visible` riêng của CommandMenu để dùng.

---

## 4.2 Option list (`p-autocomplete`/`p-select`) mặc định `white-space: nowrap; overflow: hidden`

Theme gốc (`@primeuix/styles`) đã set sẵn, KHÔNG có `text-overflow: ellipsis` — text dài tự động bị clip, KHÔNG wrap xuống dòng. `virtualScrollItemSize` cố định dùng an toàn dù item có text dài, không cần lo item wrap phá layout virtual-scroll.

---

## 4.3 `p-avatar` — `label` ưu tiên hơn `image`, truyền cả 2 thì ảnh không bao giờ hiện

Template gốc của `Avatar`: `@if (label()) {...} @else if (image()) {...}` — `label` LUÔN thắng nếu có giá trị. Component nào truyền CẢ `[image]` lẫn `[label]` cùng lúc (kiểu "ảnh nếu có, chữ tắt nếu không") thì ảnh KHÔNG BAO GIỜ hiện — bug thật gặp phải ở `indexeddb/config/components/user/user-avatar.ts` (2026-08-30, đã fix). Phải tách `@if (hasImage) { <p-avatar [image]> } @else { <p-avatar [label]> }`, không bind cả 2 song song.

---

## 4.4 Đổi cỡ/màu component PrimeNG từ NGOÀI — dùng biến, không đấu class/`::ng-deep`

Class tiện ích đặt trên host **không** đổi được kích thước component: `<p-avatar class="w-10 h-10">` vẫn ra **28px** vì `.p-avatar { width: var(--p-avatar-width) }` cùng specificity (0,1,0) nhưng nạp SAU nên thắng — trong khi `<div class="w-10 h-10">` ngay cạnh ra 35px, hai loại avatar lệch nhau mà không ai thấy (đo 2026-09-04).

`::ng-deep` từ component CHA cũng thường thua: Angular gắn `_ngcontent` vào **từng phần tử** trong style của component con, nên `:host > button i` biên dịch thành **(0,3,2)** — cao hơn hầu hết selector `::ng-deep` viết được từ cha.

**Đúng cách: set BIẾN của chính component** (`--p-avatar-width`/`--p-avatar-height`, `--p-icon-size`) ở phần tử cha bất kỳ — biến CSS thừa kế nên không phải đua specificity. Component tự viết mà muốn nơi mount đổi được thì khai biến riêng có fallback: `font-size: var(--tb-icon-size, 1.25rem)`.

⚠ Biến KHÔNG tồn tại thì `background: var(--x)` thành **invalid at computed-value time** ⇒ phần tử về `transparent`, hỏng câm. Kiểm `getComputedStyle(document.documentElement).getPropertyValue('--x')` trước khi dùng một bậc màu mới (vd `--p-primary-900`).

---

## 5. Checklist khi nâng cấp tiếp / audit lại (đã áp dụng xong cho lần migrate 2026-07-09)

```
☑ Xác nhận Angular core ≥ version PrimeNG yêu cầu — Angular 22 đã sẵn, nhưng PHẢI tự bump @angular/cdk theo cùng version (không tự động theo `ng update`/`npm install primeng`)
☑ Bump @angular/cdk khớp version primeng yêu cầu (`npm ls @angular/cdk` — peer dep "invalid" KHÔNG phải nguyên nhân chính gây lỗi build, xem mục 3 "CamelCase selector" mới là root cause thật)
☑ Quyết định licensing key trước khi coi migration hoàn tất (mục 0) — đã tích hợp qua environment.prod/local.ts (gitignored)
☑ Quyết định base font-size: đã chọn giữ compat 14px qua @primeuix/themes/material-compat
☑ Migrate pTemplate (129 file) — xong 100%, an toàn hoàn toàn
☑ So sánh prop-gap TỪNG component deprecated trước khi migrate — đã làm cho p-multiSelect/p-password/p-galleria/p-colorPicker, phát hiện gap thật ở tất cả (xem mục 2)
□ p-editor/p-chart — CHƯA quyết định licensing PrimeUI PRO, vẫn out of scope
□ Nếu upgrade tiếp lên bản v22 stable (khác RC hiện tại) → re-run toàn bộ `ng build` để xem PrimeNG có đổi API thêm gì giữa rc.2 và stable không, ĐỪNG giả định giống hệt
```

---

## 6. Tham chiếu thêm

- Angular framework upgrade (khác PrimeNG, đừng nhầm số version): `frontend/skill/skills/using-primeng-components/SKILL.md`
- PrimeNG v21 component convention hiện tại: `frontend/skill/skills/frontend-hard-rules/SKILL.md` (Stack notes)


> **Đọc file này trước khi làm bất kỳ feature FE mới nào.**
> Mục tiêu: luôn dùng API mới nhất của Angular, tránh viết code theo pattern cũ.

Phiên bản hiện tại: **Angular 22.0** (nâng từ 21.1, tháng 6/2026).
Node yêu cầu: **≥ 22.22.3 hoặc 24+** (dự án đang dùng Node 24).

---

## 1. API mới stable trong v22 — ưu tiên dùng ngay

### `apiResource()` / `rxResource()`
Thay thế pattern `ngOnInit + service.getXxx().subscribe(...)` khi load data đơn giản.

**KHÔNG dùng `httpResource()` raw** — thiếu `withCredentials` (→ 401), bypass `afterLoad()`. Dùng wrapper `apiResource()` → Xem `frontend/skill/skills/frontend-hard-rules/reference/resource-patterns.md`

```typescript
// ❌ Cũ — boilerplate, phải tự cleanup
ngOnInit() {
  this.service.getPatient(this.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe(r => {
    this.patient.set(r.data);
  });
}

// ✅ Mới — apiResource (wrapper httpResource, share/resource/api-resource.ts)
readonly patient = apiResource<Patient>(() => ({ path: `crm/patient/${this.id()}` }));
// patient.value() → data, patient.isLoading() → loading state, patient.error() → lỗi
```

**Khi nào dùng:**
- Load data theo signal param (id thay đổi → tự re-fetch)
- Thay thế `ngOnInit + subscribe` cho các GET đơn giản
- Không phù hợp: POST/PUT/DELETE, hoặc fetch cần transform phức tạp — dùng `rxResource()` hoặc service call

### `@Service` decorator
Thay `@Injectable({ providedIn: 'root' })` cho service mới, kèm rule `inject()` → Xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` (Stack notes). Service cũ giữ nguyên, không migrate hàng loạt.

### Signal debounce built-in — `.debounced()`
→ Xem `frontend/skill/skills/frontend-hard-rules/reference/resource-patterns.md` (section Signal debounce).

---

## 2. Signal Forms (stable v22)

Đã tách sang `skill/skills/building-angular-signal-forms/SKILL.md` (2026-08-10, file này vượt 350 dòng) — đọc file đó trước khi viết/convert
bất kỳ form Signal Forms nào (kết luận spike, 23 gotcha, pattern mẫu).

## 3. Breaking changes đã xử lý — KHÔNG cần làm gì thêm

| Breaking change | Trạng thái |
|---|---|
| `ChangeDetectionStrategy.Default` → `Eager` | ✅ Không ảnh hưởng — 100% component đã `OnPush` |
| `afterRender()` → `afterEveryRender()` | ✅ Không dùng trong codebase |
| `TestBed.get()` removed | ✅ Không dùng |
| Route param inheritance từ ancestor | ✅ Chỉ 1 chỗ (`app.breadcrumb.ts:149`) — đọc `.data` không phải `.params`, an toàn |
| Karma removed | ✅ Đã xử lý từ v21 |

---

## 3.1. `provideAnimationsAsync`/`provideAnimations` deprecated — CỐ Ý CHƯA MIGRATE

IDE báo `TS6385: provideAnimationsAsync is deprecated` trong `app.config.ts` — deprecated từ Angular v20.2, khuyến khích chuyển sang `animate.enter`/`animate.leave` (native CSS, bỏ `@angular/animations`), dự kiến xoá hẳn ở **v23**.

**KHÔNG xoá/migrate ngay** — PrimeNG (kể cả v22) vẫn phụ thuộc `@angular/animations`, bỏ `provideAnimations()` làm hỏng animation của `Dialog`/`OverlayPanel`/`Tooltip`/`Menubar` (xác nhận qua issue GitHub `primefaces/primeng#18863`, Open, chưa có fix/workaround — repo gốc đã archive, PrimeNG rebrand "PrimeUI" chưa migrate kịp). Còn thời gian tới v23, giữ nguyên tới khi PrimeNG tự cập nhật.

---

## 4. Suppressions hiện tại — lý do và roadmap

### NG8107 + NG8102 (suppressed trong `tsconfig.json`)
- **NG8107**: `?.` dùng thừa — TypeScript type không include `null | undefined`
- **NG8102**: `??` dùng thừa — tương tự

Suppressed vì: 2400+ occurrences, fix tay = rủi ro runtime crash nếu type không chính xác.
Fix đúng là **tighten TypeScript types** từng module, làm dần — không phải xóa `?.` cơ học.

**Roadmap fix đúng (làm theo module, không rush):**
```
1. Chọn 1 module nhỏ (vd: loyalty/broker)
2. Tìm NG8107/NG8102 warnings trong module đó
3. Với mỗi warning: xem field có thực sự nullable không
   - Nullable → thêm `| null` vào type
   - Không nullable → xóa `?.` / `??`
4. Sau khi fix 1 module → verify build + test
```

### TS5101 `baseUrl` deprecated
Thêm `"ignoreDeprecations": "6.0"` vào tsconfig. Angular CLI vẫn cần `baseUrl` cho path aliases (`@/*`).
Fix thật: migrate sang `paths` without `baseUrl` khi TypeScript 7.0 release.

### `photoswipe/style.css` (TS2882)
`src/typings.d.ts` khai báo `declare module '*.css'`. Đây là pattern chuẩn.

---

## 5. Checklist khi làm feature FE mới (Angular 22+)

```
□ Service mới → dùng @Service() thay @Injectable({ providedIn: 'root' })
□ Load data theo param → cân nhắc apiResource() thay ngOnInit+subscribe (KHÔNG httpResource raw)
□ Debounce signal → dùng .debounced(300) thay toObservable+debounceTime
□ OnPush + signals → không mutate object trực tiếp
□ KHÔNG dùng zone.js API trực tiếp (setTimeout, setInterval) — dùng rxjs hoặc signal
□ i18n: npm run i18n:find trước khi tạo key mới
```

---

## 6. Upgrade tiếp theo — Angular 23+

Angular release 1 major/6 tháng. Angular 23 dự kiến **tháng 12/2026**.

Trước khi upgrade:
1. Đọc blog.ninja-squad.com + angular.love cho release notes
2. Chạy `ng update --dry-run` để preview
3. Kiểm tra Node version yêu cầu (xu hướng tăng theo LTS)
4. Fix permission file trước khi chạy migration: `chmod -R u+w src/`

Lệnh nâng:
```bash
nvm use 24   # hoặc version LTS mới hơn
ng update @angular/core@23 @angular/cli@23
npx tsc --noEmit  # verify sau upgrade
```

---

## 7. Test builder Karma → Vitest (`@angular/build:unit-test`)

- Builder cần thấy literal `"zone.js"` trong mảng `polyfills` của build target để chọn strategy `'static'` (import thường). Nếu app dùng file polyfills custom (`"src/polyfills.ts"`) thay vì literal `"zone.js"`, builder rơi vào `'dynamic'` (top-level await) → fail build test với lỗi "Top-level await is not available".
- Fix: thêm 1 build configuration riêng cho test (không đụng production) với `polyfills: ["src/polyfills.ts", "zone.js"]`, trỏ `test.options.buildTarget` sang configuration đó.
- Cần cài thêm `jsdom` làm devDependency — không tự bundle kèm vitest.


`environment.option: 1 | 2 | 3` trong mỗi file `environment*.ts` — điều khiển module nào hiển thị cho client.

## Tier mapping

| Option | Modules |
|--------|---------|
| 1 | CRM, Chăm sóc KH, Nhân sự, Kế toán |
| 2 | Option 1 + Sale, Kho & Vật tư, EMR |
| 3 | Tất cả (thêm: Loyalty, Dược & Vật tư, Marketing, Zalo, Cấu hình) |

## Implementation

**Home** (`src/app/modules/utilities/home/home.ts`):
- `ModuleDef.minOption?: number` — tier tối thiểu để hiển thị module
- Filter trong `computed`: `environment.option < (m.minOption ?? 1)` → ẩn

**Topbar** (`src/app/layout/components/app.topbar.ts`):
- `protected readonly envOption = environment.option`
- Template dùng `@if (envOption >= 2)` / `@if (envOption >= 3)` bao quanh từng module block

## Mobile (khác web: KHÔNG build-time)

Mobile 1 app cho mọi tenant → tier đọc runtime: option `erpOption` (1|2|3, default 3) trong config_systems (`dexie.ConfigSystemsSetOption(db, define.OptErpOption, v)`) → `/api/mobile/v1/ping` trả `erp_option` → app persist + filter module theo `minOption` (mirror computed Home web). Đổi tier: sửa option, hiệu lực ≤1 phút, không deploy.

## Thay đổi tier cho client

Chỉ sửa `option` trong `environment.prod.ts` của client đó:
```ts
export const environment = {
  prod: true,
  option: 2,  // ← đổi 1 | 2 | 3
  apiUrl: '/api',
  isDental: false,
};
```

## Thêm module mới vào tier system

1. **Home**: thêm `minOption: <1|2|3>` vào object trong `allModules`
2. **Topbar**: bọc `<a routerLink="...">` bằng `@if (envOption >= <n>) { ... } <!-- comment -->`

## File nào tracked git, file nào không

`environment.ts` (base, dùng khi `ng serve` không có `-c`) — TRACKED git.
`environment.prod.ts`/`environment.local.ts` (dùng qua `fileReplacements` trong `angular.json` cho `-c production`/`-c local`) — GITIGNORED.
Secret/API key/license key → đặt ở `environment.prod.ts`/`environment.local.ts`, để rỗng `''` ở `environment.ts`. KHÔNG hardcode secret trực tiếp vào `.ts` khác (vd `app.config.ts`) — mọi file `.ts` ngoài `environments/` đều tracked git.
