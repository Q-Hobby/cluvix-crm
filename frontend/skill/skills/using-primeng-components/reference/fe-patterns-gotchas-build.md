# Gotchas Build/Tooling/Git

## Mục lục

- `tsc --noEmit` không bắt lỗi type-check template Angular
- `ng build` ghi đè thư mục git-tracked — `--output-path` KHÔNG bảo vệ được (CRITICAL)
- `frontend/` có `.git` riêng biệt với repo root
- acceptButtonStyleClass silently no-op (ConfirmPopup/ConfirmDialog)
- Vite dep-cache stale → 504 Gateway Timeout, lazy route đứng
- Regex bulk-edit HTML: attribute chứa `>`/`>=` phá vỡ tag-boundary
- Script khảo sát codebase: 2 lỗi làm kế hoạch sai mà build vẫn xanh
- File vừa sửa "biến mất" giữa session — check git rename trước khi báo lỗi
- `tsc` với tsconfig tạm KHÔNG có `rootDir` → emit `.js` rải khắp `src/`
- `ng build` FAIL vẫn XOÁ SẠCH `public/` của repo cha (2667 file git-tracked)
- Dynamic field-key binding trong @for: 2 lỗi chỉ `ng build` bắt được
- Tailwind `group-hover` cần quan hệ CHA–CON, không phải anh em (CRITICAL)
- Verify UI ngay sau khi sửa file FE → dev-server rebuild làm SAI kết quả
- Backtick trong comment CSS/HTML bên trong `styles: [` … `]` phá vỡ template literal
- Một spec hỏng chặn TOÀN BỘ suite `ng test` — `--include` không cứu được
- [`vitest-base.config.ts` không biên dịch Angular — spec dùng `setInput` đỏ giả](#vitest-baseconfigts-không-biên-dịch-angular--spec-dùng-setinput-đỏ-giả)
- `vitest run` im lặng bỏ qua spec khi biến path bị zsh nuốt từ
- Mutation FE (vitest) phải giữ code compile — dùng `void x`, không xoá biến
- `@keyframes` khai trong component KHÔNG dùng được với `[style.animation]` (Angular đổi tên)
- `background-position` phần trăm tính theo PHẦN TỬ — sai công thức sprite animation
- Hằng lớn nằm chung file với export bị import lúc boot ⇒ luôn vào bundle eager
- [Không viết được spec cho component dùng `FormShareImports` (chặn cả repo)](#không-viết-được-spec-cho-component-dùng-formshareimports--chặn-cả-repo)
- [`tsc` + `ngc` xanh vẫn ra bảng rỗng — gate không bắt được lỗi DÙNG SAI API](#tsc--ngc-xanh-vẫn-ra-bảng-rỗng--gate-không-bắt-được-lỗi-dùng-sai-api)

---


Tách từ `frontend/skill/skills/working-on-fe-modules/reference/fe-patterns-gotchas.md` (giữ file chính ≤ 350 dòng).

## `tsc --noEmit` không bắt lỗi type-check template Angular

`npx tsc --noEmit` chỉ check `.ts`, KHÔNG check binding trong `.html` (AOT template type-checking).
Trước khi báo "xong" cho thay đổi FE có đổi template → chạy `npx ngc -p tsconfig.app.json --noEmit`: check đủ template, KHÔNG ghi ra file nào.
Kể cả khi KHÔNG tự đổi template nào — nâng version thư viện UI bên thứ 3 (PrimeNG...) vẫn phải verify AOT, vì thư viện có thể đổi API mà tsc không bắt được (vd đổi tên component selector).

**Loại lỗi ngc bắt mà tsc bỏ qua HOÀN TOÀN** — viết template mới là dính, không cần nâng version gì:
- `NG8001: '<tên>' is not a known element` — **sai hoa/thường trong selector**. PrimeNG v22 dùng chữ thường liền: `p-progress-spinner` (KHÔNG `p-progressSpinner`), `p-inputnumber` (KHÔNG `p-inputNumber`). Sai thì `tsc` xanh, thẻ render rỗng, màn hình thiếu control mà không lỗi nào.
- `NG8002: Can't bind to 'X' since it isn't a known property` — hệ quả của cùng lỗi trên (`[min]`/`[max]` đặt trên selector không tồn tại), hoặc component chưa khai trong `imports`.
- Component dùng nhưng **chưa có trong `imports`** — vd `MessageModule` KHÔNG nằm trong `FormShareImports`, phải tự import khi dùng `p-message`.

Tra selector đúng bằng chính repo: `grep -rho "p-<tên>" src/app/modules/ | sort | uniq -c` — dạng nhiều hit nhất là dạng đang chạy thật. Đo 2026-09-06: cả ba dạng `p-progress-spinner` (7) / `p-progressspinner` (5) / `p-progressSpinner` (1) cùng tồn tại trong repo, chỉ hai dạng đầu render được.
Hàng loạt cảnh báo `NG8113: X is not used within the template` cho MỌI import của 1 component = template component đó **parse hỏng** (thường kèm `NG5002` ở dòng dưới), KHÔNG phải import thừa. Đọc `NG5002` trước, đừng đi gỡ import.

⚠ **`ngc --noEmit` đang exit 1 SẴN** (19 lỗi đo 2026-08-24; **17 lỗi** đo lại 2026-09-06 ở `image-comparison`, `money-shorthand.directive`, `notification-item` (component đã bỏ) — đều là code đã commit. Con số này TRÔI theo thời gian ⇒ tự đo lại mỗi lần, đừng lấy số trong tài liệu làm mốc) ⇒ **KHÔNG dùng exit code làm cổng pass/fail**. Cách dùng đúng: đếm số lỗi TRƯỚC và SAU khi sửa (chênh lệch mới là của mình) + lọc theo path file mình đụng. Output có màu → bóc ANSI (`\x1b\[[0-9;]*m`) trước khi grep, nếu không `grep "error TS"` trả 0 giả. ⚠ Và pattern phải là `grep -cE "error (TS|NG)"`: lỗi TEMPLATE mang mã `NG8xxx` (`NG8002`...), `grep "error TS"` bỏ lọt sạch — xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` §B.11 (đo 2026-09-21). Cách này đã bắt được 1 regression thật: gỡ `ListShareImports` khỏi 1 trang in làm mất directive `[appCopyText]`, 19→20 lỗi.

---

## `ng build` ghi đè thư mục git-tracked — `--output-path` KHÔNG bảo vệ được (CRITICAL)

`angular.json` có `outputPath` trỏ tới `/Users/KemBi/www/erp-cluvix/public` — đây là thư mục **git-tracked**, không phải build tạm.
`ng build --output-path <scratchpad>` **VẪN dọn sạch `public/`** — CLI clean `outputPath` cấu hình trong `angular.json` bất kể cờ dòng lệnh (đã xoá 723 file tracked, 2026-08-10). Không có cờ nào né được → dùng `ngc --noEmit` để verify AOT, KHÔNG chạy `ng build`.
⚠ Đo NGƯỢC LẠI trên `@angular/build` **22.0.4** (2026-08-27): `--output-path` ra thư mục ngoài KHÔNG dọn `outputPath` — 1256 file chưa-theo-dõi đề 24/08 trong `public/` sống sót qua 2 lượt build, và asset mới không lọt vào `public/`. Có thể phụ thuộc phiên bản, hoặc lượt 2026-08-10 chạy không kèm cờ. Đo lần 3 (2026-08-28, `@angular/build` 22.x, 4 lượt build liên tiếp có/không `--source-map`): `--output-path` đường dẫn TUYỆT ĐỐI KHÔNG dọn `public/` — mtime file cũ giữ nguyên, `git status public/` không đổi. Thành 2 đo thuận / 1 đo nghịch. **Vẫn giữ `ngc --noEmit` làm mặc định** — sai ở chiều kia mất 2667 file git-tracked, không đáng đánh cược.
`ng serve` chạy nền cũng ghi liên tục vào `public/` → `public/` bẩn trong `git status` thường là dev server chứ không phải mình; KHÔNG `git checkout -- public/` theo phản xạ.
KHÔNG BAO GIỜ `rm -rf public` (hay bất kỳ output-path nào) mà chưa `git status` trước.

**Mức nghiêm trọng — user chốt 2026-08-24: `public/` bẩn KHÔNG phải sự cố cần cứu.** Đây là artifact build, user build lại là xong. Lỡ chạy `ng build` làm bẩn/xoá `public/` → **báo user rồi đi tiếp**, TUYỆT ĐỐI không tìm cách khôi phục bằng `git checkout`/`git restore`/ghi lại từng file từ `HEAD` (hook `guard-bash.sh` chặn, và làm vậy có nguy cơ đụng việc chưa commit của phiên khác). Rule dùng `ngc --noEmit` ở trên vẫn giữ nguyên — nó là cách verify đúng, không phải vì sợ hỏng `public/`.

---

## `frontend/` có `.git` riêng biệt với repo root

Git operations (diff/status/log) trên file FE PHẢI chạy với cwd trong `frontend/`, không phải root/`backend/` — chạy sai cwd không báo lỗi, chỉ trả kết quả rỗng/im lặng.

---

## acceptButtonStyleClass silently no-op (ConfirmPopup/ConfirmDialog)

PrimeNG v17+: Button đọc severity/size qua input, KHÔNG đọc CSS class string nữa.
`acceptButtonStyleClass: 'p-button-danger p-button-sm'` không báo lỗi nhưng KHÔNG có tác dụng.
✅ Dùng: `acceptButtonProps: {severity: 'danger', size: 'small'}`

---

## Vite dep-cache stale → 504 Gateway Timeout, lazy route đứng

Angular 22 dev-server (`@angular/build:application`) dùng Vite pre-bundle deps vào `.angular/cache/**/vite/deps/*.js`.
Cache stale (dev server bị kill không graceful, hoặc `npm install` khi server đang chạy) → `504` khi fetch 1 dep (vd `echarts.js`) → mọi route lazy-load module đó fail dynamic import, chỉ trang không lazy-load (Home) còn dùng được.
✅ Fix: `rm -rf .angular/cache` rồi chạy lại `ng serve`. Script tiện: `npm run startLocal:clean`.

---

## Regex bulk-edit HTML: attribute chứa `>`/`>=` phá vỡ tag-boundary

Script quét/sửa hàng loạt bằng regex `<tag\b[^>]*?(/>|>...)` KHÔNG track quote-state — 1 binding kiểu `[disabled]="x >= y"` bên trong attribute value khiến `[^>]` coi dấu `>` đó là điểm đóng thẻ, bỏ sót cả thẻ.
✅ Sau khi chạy regex-scan, LUÔN chạy lại 1 lượt quote-aware scan (duyệt ký tự, track `"`/`'` mở/đóng) để đối chiếu — đừng tin tuyệt đối kết quả regex đơn giản trên HTML có Angular binding.

---

## Script khảo sát codebase: 2 lỗi làm kế hoạch sai mà build vẫn xanh

Đúc từ epic `crm-list-v2` (2026-08-03) — cả hai đều lọt tới khi Dev thực thi mới lộ.

**Khai báo class xuống dòng** — regex 1 dòng `export class (\w+) extends (BaseX)` bỏ sót:
```ts
export class CheckUpOfCustomer
  extends BaseList<ToothBioModel>     // ← regex 1 dòng không khớp
```
✅ `export class (\w+)\s*(?:<[^>]*>)?\s*\n?\s*extends\s+BaseX\b`. Sót 9/56 component vì lỗi này.

**Dò selector bằng chuỗi con** — `'<app-check-up-list' in html` khớp nhầm `<app-check-up-list-in-customer-detail` (component KHÁC).
✅ Chốt ranh giới: `re.search(r'<' + re.escape(sel) + r'[\s/>]', html)`.

Kiểm chứng lại tổng số bằng cách thứ 2 trước khi chốt phạm vi epic — đếm sai đầu vào thì mọi wave/story sau đều lệch.

## File vừa sửa "biến mất" giữa session — check git rename trước khi báo lỗi

User có thể đang sửa/di chuyển file song song qua IDE (GoLand...) trong khi Claude Code cùng thao tác trên working tree — file vừa Read/Edit thành công có thể bị move/rename ngay sau đó bởi phía user, khiến `ls`/build báo "not found"/lỗi import.
✅ Trước khi kết luận "lỗi lạ"/tự sửa: `git status --short <path>` — nếu thấy `R`/`RM` (rename) thay vì `D` đơn thuần, đó là move hợp lệ, không phải hỏng. Hỏi user xác nhận thay vì tự đoán hoặc tự phục hồi.

## `tsc` với tsconfig tạm KHÔNG có `rootDir` → emit `.js` rải khắp `src/`

Gặp thật khi muốn chạy thử một hàm thuần (epic `care-list-v2`, story-00, 2026-08-03): Dev tạo tsconfig
tạm để biên dịch 2 file nguồn ra JS rồi chạy `node`. Thiếu `rootDir`/`outDir` → `tsc` **báo lỗi NHƯNG VẪN
EMIT**, rải 87 file `.js` nằm cạnh chính file `.ts` trong `src/`.

Nguy ở chỗ: `.js` trong `src/` **không được git track** nên `git status` trông vẫn sạch, mà `ng build`
/dev-server lại có thể pick nhầm. Dọn xong khó biết đã sạch chưa.

✅ Muốn chạy thử hàm thuần: đặt tsconfig tạm + output **ngoài repo** (thư mục scratchpad của session),
KHÔNG bao giờ để `tsc` ghi vào `src/`. Luôn khai `noEmit: true` nếu chỉ cần type-check.
✅ Kiểm tra sau khi lỡ chạy: `find src/app -name "*.js"` phải TRỐNG (`src/assets/**` và
`src/custom-ngsw-worker.js` là `.js` hợp lệ có sẵn, đừng xoá nhầm).
✅ `tsc` báo lỗi ≠ `tsc` không ghi file — muốn chắc chắn không ghi thì phải `--noEmit`, không dựa vào exit code.

## `ng build` FAIL vẫn XOÁ SẠCH `public/` của repo cha (2667 file git-tracked)

Angular dọn `outputDir` **trước** khi biên dịch. Build hỏng giữa chừng → output cũ đã mất, output mới
không sinh ra → `public/` (git-tracked ở repo cha `erp-cluvix`) trống rỗng, `git status` repo cha hiện
hàng nghìn dòng `D`.

Nguy nhất khi **fan-out nhiều phiên Claude Code**: phiên A chạy `ng build` để verify, nhưng phiên B đang
sửa dở một component ở module khác → build fail vì lỗi của B, mà `public/` thì A đã xoá mất.
Gặp thật ở epic `care-list-v2` (2026-08-03): build hỏng ở `config/` của phiên khác, xoá 2667 file.

✅ Build THÀNH CÔNG cũng làm bẩn, và bẩn kiểu khác: file tracked bị `M`/`D` **cộng thêm** hàng nghìn file
`??` (dev build sinh tên asset trần `primeicons.woff`, bản prod đã commit dùng tên có hash
`primeicons-4GST5W3O.woff2`). `git restore` KHÔNG dọn được `??` → phải `git clean -nd public` xem
trước, đối chiếu đúng toàn asset build rồi mới `git clean -fdq public`. Bỏ bước này là commit sau
cuốn theo bản dev build đè lên bản production. Kèm theo: prebuild stamp `LS_VERSION` trong
`cached-translate-loader.ts` cũng đổi mỗi lần build — khôi phục nốt nếu không định commit.
✅ Khôi phục: `git checkout -- public/` ở **repo cha**. An toàn khi `git status public/` chỉ toàn `D`
(không `M`) — nghĩa là không ai có thay đổi chưa commit trong đó. Kiểm phân loại trước:
`git status --short public/ | awk '{print $1}' | sort | uniq -c`.
✅ Trước khi `ng build` lúc đang fan-out: xác định build fail có phải lỗi của mình không. `npx tsc --noEmit`
sạch mà `ng build` hỏng ở module mình không đụng → là phiên khác, **đừng build lại nhiều lần**, và nhớ
khôi phục `public/`.
✅ `ng build` KHÔNG phải thước đo dùng được khi cây mã đang có phiên khác gõ dở — build dừng ở lỗi đầu
tiên nên cũng không chứng minh được phần của mình sạch AOT. Thay bằng phép kiểm nhắm đúng loại lỗi
(vd: mọi hàm template gọi phải tồn tại trong TS/base).

## Dynamic field-key binding trong @for: 2 lỗi chỉ `ng build` bắt được

Angular template KHÔNG hỗ trợ computed property `{ [f.key]: $event }` trong object literal
(NG5002 Parser Error: Unexpected token [) — dùng method riêng: `patchField(key, value)` trong .ts,
gọi `(ngModelChange)="patchField(f.key, $event)"` trong template.

`keyof` trên interface có index signature `[key: string]: unknown` suy ra `string | number`
(không phải `string` thuần) — `[inputId]="f.key"` vỡ AOT (`inputId` cần string). Fix: khai type
field key bằng `Extract<keyof T, string>` thay vì `keyof T` thô.

## Verify UI ngay sau khi sửa file FE → dev-server rebuild làm SAI kết quả

KHÔNG thao tác/verify trên browser ngay sau khi vừa Edit file FE. `ng serve` live-reload có thể nổ **đúng lúc đang click**: trang remount, form mất dữ liệu, request KHÔNG bắn — nhìn y hệt một bug thật của tính năng.

- Đợi rebuild xong rồi mới thao tác (`ng serve` báo compiled; phía BE: `backend/tmp/run.log` của `air`).
- Nghi ngờ → đối chiếu log BE trước khi kết luận: **không có request nào trong log = client chưa từng gửi**, không phải BE từ chối. Từ đó mới tách được "bug tính năng" với "nhiễu do reload".
- Ca thật 2026-08-12: bấm "Lưu kết luận" ngay sau khi sửa 5 file → bệnh án không được tạo, đã báo thành finding; thực chất là HMR reload, chạy lại trên build ổn định thì `POST → 201` bình thường.


---

## Backtick trong comment CSS/HTML bên trong `styles: [` … `]` phá vỡ template literal

Comment giải thích trong `styles: [\`...\`]` (và trong `template: \`...\`) mà chứa dấu backtick — kiểu
viết `` `white-space: nowrap` `` để nhấn tên thuộc tính — sẽ **đóng sớm template literal**. Lỗi báo là
`TS1005: ',' expected` ở một dòng **cách đó rất xa**, không hề nhắc tới backtick, nên rất dễ đi tìm nhầm chỗ.

Xảy ra 4 lần trong một phiên (2026-08-21), và một phiên Claude Code khác mắc cùng lỗi ở
`omnichat-sidebar.ts` cùng ngày. Quy ước: **trong `styles`/`template` viết tên thuộc tính CSS trần**
(`white-space: nowrap`), không bọc backtick.

```ts
// ❌ vỡ build, lỗi báo ở dòng khác hẳn
styles: [`
  /* KHÔNG dùng `white-space: nowrap`: nhãn dài sẽ tràn */
  .x { ... }
`]

// ✅
styles: [`
  /* KHÔNG dùng white-space: nowrap: nhãn dài sẽ tràn */
  .x { ... }
`]
```

## `@keyframes` khai trong component KHÔNG dùng được với `[style.animation]` (Angular đổi tên)

Angular scope-hoá `@keyframes` khai trong `styles: []` của component — tên thật trong stylesheet trở
thành `_ngcontent-ng-cXXXXXXXX_<tên>`. Nếu animation-name được set bằng **inline style binding**
(`[style.animation]="'my-anim 1s steps(5) infinite'"`) thì nó dùng **tên gốc** ⇒ không khớp tên đã
scope ⇒ trình duyệt **không chạy animation nào**.

Nguy hiểm vì mọi tín hiệu bề mặt đều "xanh": build pass, `getComputedStyle(el).animationName` trả về
đúng tên gốc, `animation-play-state` báo `running`. Cách phát hiện thật: **`el.getAnimations()` trả
mảng RỖNG** (đo 2026-08-21, sticker động omnichat).

- Animation áp bằng **CSS class trong cùng stylesheet component** → không sao (cả rule lẫn keyframes
  đều được rename nhất quán). Ví dụ đang chạy đúng: `.omni-jump-target` trong `omnichat-thread.ts`.
- Animation có tên đến từ **TS/inline binding** → khai `@keyframes` ở **global** (`src/styles.scss`).
  Ví dụ: `omni-sticker-play`.

```ts
// ❌ keyframes trong component + tên set qua binding → im lặng không chạy
styles: [`@keyframes my-anim { ... }`]
// template: [style.animation]="'my-anim 1s infinite'"

// ✅ keyframes ở src/styles.scss, tham số truyền qua CSS var
// template: [style.animation]="'my-anim 1s infinite'" + [style.--my-var]="..."
```

## `background-position` phần trăm tính theo PHẦN TỬ — sai công thức sprite animation

Sprite animation (`steps(N)` chạy một dải khung ngang) thường được viết với mốc cuối là phần trăm.
**Sai**: `background-position: X%` cho offset `(bề rộng phần tử − bề rộng ảnh) × X%` — phần trăm tính
theo *phần tử*, không theo sprite. Đo bằng thực nghiệm (sprite 3 khung, khung 130px):

| Mốc cuối | Kết quả |
|---|---|
| `-100%` | offset thành DƯƠNG → sprite bị đẩy ra ngoài, ô **trống trơn** |
| `100%` | dừng ở −173px trong khi biên khung là −130/−260 → thấy **hai nửa khung** |
| `-(số khung × cạnh)px` | ✅ đúng một khung nguyên vẹn mỗi bước |

⚠ `-100%` là cách repo tham khảo ZCRM-EE dùng (`message-bubble.vue` `.sticker-anim`) — chép nguyên
sang là mang theo lỗi của họ. Truyền mốc px qua CSS var vì nó phụ thuộc số khung từng sprite.

Lưu ý khi đo: `getComputedStyle` KHÔNG phản ánh giá trị đang animate, và `Animation.currentTime` đứng
yên khi tab bị ẩn (`document.visibilityState === 'hidden'`, rAF không chạy) — đứng yên ở đó KHÔNG có
nghĩa animation hỏng. Muốn xem từng khung: `anim.pause()` rồi gán `anim.currentTime` và chụp màn.

## Hai cái bẫy IM LẶNG khi viết CSS trong component Angular

Cả hai đo được ngày 2026-08-25 (omnichat reaction), đều thuộc loại "trình duyệt/compiler bỏ qua mà
không kêu một tiếng".

### 1. `calc()` trên biến CSS là shorthand NHIỀU GIÁ TRỊ ⇒ CSS không hợp lệ, bị bỏ im lặng

`omnichat-thread.ts` khai `--omni-bub-pad: 7px 11px` (shorthand 2 giá trị). Viết
`calc(var(--omni-bub-pad) + 6px)` thành ra `calc(7px 11px + 6px)` — **không phải CSS hợp lệ**, trình
duyệt bỏ nguyên dòng khai báo. Không lỗi, không cảnh báo, không có gì trong DevTools trừ khi soi đúng
thuộc tính đó. Hậu quả ở ca thật: padding không tăng ⇒ nút nổi đè lên giờ gửi.

**Luật:** biến CSS dùng để `calc()` phải là **một giá trị đơn**. Cần shorthand thì tách:
```css
--omni-bub-pad-y: 7px;  --omni-bub-pad-x: 11px;
--omni-bub-pad: var(--omni-bub-pad-y) var(--omni-bub-pad-x);   /* dùng nguyên khối */
--omni-bub-pad-bottom-react: calc(var(--omni-bub-pad-y) + 6px); /* dùng để tính */
```

### 2. Dấu huyền (backtick) trong comment CSS ĐÓNG luôn template literal của `styles: [...]`

`styles: [\`...\`]` là template literal TS. Một dấu huyền trong comment CSS (kiểu quen tay viết
`/* biến \`--foo\` là… */`) kết thúc chuỗi ngay tại đó ⇒ `tsc` nôn ra hàng loạt `TS1005 ',' expected`
ở **những dòng chẳng liên quan gì**, và người đọc đi tìm lỗi cú pháp ở sai chỗ hoàn toàn.

**Luật:** trong khối `styles` KHÔNG dùng dấu huyền — viết tên biến trần hoặc bọc dấu nháy đơn.

### 3. `[style.padding]` + `[style.padding-bottom]` chạy được nhưng đừng dựa vào

Angular ưu tiên thuộc tính CỤ THỂ hơn shorthand nên cặp này ra đúng kết quả — nhưng đó là luật người
đọc sau phải BIẾT mới dám sửa, và sai thì hỏng im lặng. Ghép sẵn chuỗi trong một hàm rồi bind MỘT
lần (`[style.padding]="bubblePadding(msg)"`) thì nhìn là hiểu, không cần biết luật nào.

---

## Tailwind `group-hover` cần quan hệ CHA–CON, không phải anh em (CRITICAL)

`group-hover:` compile ra selector **con cháu** (`.group:hover .group-hover\:opacity-100`) — phần tử mang
`group` phải là **tổ tiên** của phần tử ẩn/hiện. Đặt `group` ở phần tử **anh em** thì quy tắc không bao
giờ khớp: chỉ `focus-within` (bàn phím) còn hiện được, chuột thì không.

Hỏng loại này KHÔNG bị `tsc`/lint/test hàm thuần bắt. Neo bằng test đọc DOM thật và khẳng định quan hệ
`contains()` giữa phần tử `.group` và phần tử ẩn/hiện.

## Một spec hỏng chặn TOÀN BỘ suite `ng test` — `--include` không cứu được

`ng test` biên dịch **cả dự án**: một file `*.spec.ts` lỗi type làm **toàn bộ** suite không chạy, kể cả spec không liên quan.
`--include=<glob>` KHÔNG cứu được — nó lọc lúc CHẠY, không lọc lúc BIÊN DỊCH.
Spec phải `import { describe, it, expect } from 'vitest'` tường minh; repo không bật globals.
Suite đỏ toàn bộ với lỗi `TS2593: Cannot find name 'describe'` / `TS2304: Cannot find name 'expect'` ⇒ tìm file spec **chưa commit mới nhất** trước khi nghi code của mình.

---

## `vitest-base.config.ts` không biên dịch Angular — spec dùng `setInput` đỏ giả

`vitest-base.config.ts` không có plugin ngtsc ⇒ `OmnichatThread.ɵcmp.inputs` = `{}` ⇒
`ComponentRef.setInput()` trên signal input **im lặng không set** (chỉ log NG0303 async), component render
trạng thái rỗng, assert DOM đỏ dù code đúng. Tái hiện 2026-09-05 trên 4 spec `omnichat/thread/*`
(`forward-label`, `reply-target-reset`, `pin-bar`, `partial-retry`): vitest-base 7/11, `ng test` 11/11.
- Spec render component + `setInput` ⇒ `npx ng test --include='<spec>' --watch=false`.
- `npx vitest run -c vitest-base.config.ts` chỉ cho hàm thuần / service không render.
- Đỏ dưới vitest-base mà xanh dưới `ng test` ⇒ KHÔNG phải lỗi spec; đừng "sửa" spec cho vừa runner.

## `vitest run` im lặng bỏ qua spec khi biến path bị zsh nuốt từ

Truyền nhiều đường dẫn spec qua một biến shell nhiều dòng trần (`SPECS="a.spec.ts\nb.spec.ts"; npx vitest run -c vitest-base.config.ts $SPECS`) — zsh KHÔNG tách từ theo dòng, vitest nhận đúng MỘT chuỗi dài, không khớp file nào ⇒ `No test files found`, exit code vẫn phản ánh "0 test = không có gì hỏng". Dùng mảng (`${=SPECS}` nếu buộc phải dùng biến trần trong zsh) hoặc liệt kê từng path riêng trên dòng lệnh.

**Luôn đối chiếu `Test Files N passed` với đúng số spec đã truyền** — con số thấp hơn kỳ vọng là dấu hiệu path bị nuốt/gõ sai, không phải "một vài spec không cần chạy".

## Mutation FE (vitest) phải giữ code compile — dùng `void x`, không xoá biến

Xoá thẳng biến/nhánh đang mutate để "tắt" nó thường làm `tsc`/biên dịch FAIL trước khi vitest kịp chạy ⇒ **0 đỏ giả** (không phải test bắt được, mà cả suite không build được). Giữ compile bằng cách thay logic bằng no-op tham chiếu biến (`void toSignal`, `void OS_NOTIFY_CHANGED_EVENT;`) rồi mới đọc kết quả — chỉ tin "N failed" khi các mutation khác trong cùng lượt (chưa mutate) vẫn pass, chứng minh lưới CÓ chạy chứ không phải build vỡ.

---

## Hằng lớn nằm chung file với export bị import lúc boot ⇒ luôn vào bundle eager

esbuild phát mỗi module **đúng một lần, ở chunk cha chung**. Module nào bị code chạy-lúc-boot
(guard, menu, interceptor) import thì nằm ở `main`; hằng lớn export trong CÙNG file đó bị kéo theo
**kể cả khi chỉ một route lazy dùng tới** — tree-shaking không cứu, vì export đó có người dùng thật.

Tách hằng sang **file riêng** và import **trực tiếp theo path**, KHÔNG re-export qua barrel
(`@/define`) — đi qua barrel là kéo ngược vào eager.

```ts
// ❌ permission.define.ts: PERMISSION_SELECT (guard dùng, eager) + PERMISSION_MATRIX (149 module)
// ✅ permission-matrix.define.ts riêng; user-role.ts (lazy) import thẳng path, không qua '@/define'
```

Verify trên bundle đã build: grep marker chỉ-có-trong-hằng ở `main-*.js`, **kèm một marker đối chứng
chắc chắn có** để chứng minh lệnh grep thật sự chạy. Đo thật (2026-08-28): main 3586 → 3401 KB raw,
797 → 769 KB gzip.

---

## Không viết được spec cho component dùng `FormShareImports` — chặn cả repo

Đo 2026-09-09. Spec chỉ cần **một dòng** này là đỏ, chưa cần import component nào:

```ts
import { FormShareImports } from '@/share/import/form-share-imports';
// → TypeError: Class extends value undefined is not a constructor or null
//   ❯ src/app/core/services/auth/auth.service.ts:21
```

- Nguyên nhân: import vòng — `auth.service` `extends BaseApiService`, `BaseApiService` chưa eval xong
  khi barrel kéo cả cây service vào.
- `vi.mock('@/share/import/form-share-imports', …)` **KHÔNG chặn được** dưới builder
  `@angular/build:unit-test` (mock theo alias `@/` không khớp module thật).
- `FormShareImports` là import chuẩn của mọi màn hình ⇒ **hiện không có component nào test được**;
  chỉ test được logic thuần đã tách ra file `util`.
- Hệ quả với quy trình: story FE khai "AC X có test" phải kiểm lại — nếu AC đó nằm trong component,
  nó chỉ được xác minh bằng đọc code + smoke tay. Nói rõ ra, đừng để người sau tưởng có lưới.
- Chạy `npx ng test` toàn bộ hiện có ~11 spec đỏ do chạy SONG SONG (chạy riêng từng file thì xanh) —
  đừng nhầm nhóm này với lỗi thật của mình.

---

## `tsc` + `ngc` xanh vẫn ra bảng rỗng — gate không bắt được lỗi DÙNG SAI API

Đo 2026-09-09, lộ ra ở lần smoke đầu tiên trên production.

Màn mới dùng `<ng-template pTemplate="header">` cho `p-table`. Kết quả: thẻ thống kê hiện đúng số,
còn **bảng render rỗng hoàn toàn** — không header, không dòng, không cả empty state, **không lỗi
console**. Nguyên nhân: v22 chỉ đọc `#header`/`#body`/`#expandedrow`/`#emptymessage`.

Vì sao cả 2 cổng đều xanh: `pTemplate` **là directive có thật, hợp lệ về kiểu**. Sai ở chỗ component
không còn đọc nó — đó là lỗi **ngữ nghĩa API**, không phải lỗi cú pháp. `ngc` chỉ kiểm được cú pháp
và kiểu.

⇒ Ba hệ quả, áp cho mọi lần dùng API của thư viện:

1. **Tự kiểm gate bằng cách cấy lỗi CÚ PHÁP (`[valueXX]`) chỉ chứng minh gate CÓ CHẠY** — không
   chứng minh gate bắt được lớp lỗi này. Đừng suy từ "gate đã tự kiểm" ra "gate đủ".
2. **Đếm quy ước của repo trước khi viết theo trí nhớ.** Một lệnh, trước khi gõ dòng đầu tiên:
   ```bash
   grep -rho '<ng-template #[a-z]*' --include='*.html' src/app | sort | uniq -c | sort -rn
   ```
   Mình là file thiểu số (1 vs 440) thì mình sai, không phải repo sai.
3. **Rule có sẵn mà không tìm ra cũng bằng không có.** Rule này đã nằm trong
   `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-form.md` từ trước — nhưng ở skill
   FORM, trong khi lỗi xảy ra lúc dựng BẢNG. Viết component loại X thì rule phải với tới được từ cửa
   vào của loại X.

Cùng lớp với "một cột chỉ hiển thị sai mà không ai đỏ": lỗi **im lặng ở tầng trình bày** không có
cổng tự động nào bắt được — chỉ smoke bằng mắt mới thấy. Đó là lý do checklist smoke không bỏ được.
