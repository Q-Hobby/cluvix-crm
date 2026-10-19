---
name: building-angular-signal-forms
description: >-
  Angular Signal Forms (BaseSignalForm) trong Cluvix: kết luận spike, gotcha #1–#55,
  pattern mẫu, khuôn form nhúng (không dialog). Dùng skill này TRƯỚC KHI viết form
  mới hoặc convert form BaseForm → BaseSignalForm — đặc biệt trước khi đụng
  (ngSubmit), merge logic, hoặc p-table binding trong form, vì các bug ở đây từng
  gây mất sạch dữ liệu form dù build vẫn xanh.
---

# Building Angular Signal Forms

> Gotcha chi tiết #15–#55: `reference/signal-forms-gotchas-2.md` · Gotcha Form & List:
> `reference/fe-patterns-gotchas-form.md` · UX form: `reference/frontend-forms-ux.md` ·
> Gotcha Signals & CD: `reference/fe-patterns-gotchas-signals.md` · AppConfirmDialog:
> `reference/fe-patterns-confirm-dialog.md`



> Đọc file này trước khi viết/convert bất kỳ form Signal Forms nào (`BaseSignalForm`).

Signal Forms đã stable nhưng **chưa ưu tiên migrate** codebase hiện tại vì:
- Toàn bộ forms đang dùng Reactive Forms (`FormControl`, `FormGroup`) — hoạt động tốt
- Migration hàng trăm form (214 form `extends BaseForm`) = rủi ro cao, lợi ích chưa rõ
- Validator `min`/`max` trong Signal Forms chỉ nhận `number` (không nhận string)

**Áp dụng cho form mới** nếu đơn giản và team đã quen signals. Không bắt buộc.

## Kết luận spike `signal-forms-poc` — CÓ BẰNG CHỨNG cụ thể

Đã dựng `BaseSignalForm` (`src/app/share/base/base-signal-form.ts`) làm base độc lập cho form Signal
Forms (song song `BaseForm`, không sửa) và convert `ServiceForm` — form phức tạp nhất repo (570 dòng,
7 tab, `FormArray`, dynamic validator, cross-field, upload ảnh) sang Signal Forms. Test tay thật trên
browser (dev server `ng serve`, không chỉ đọc code). **Đã lên production thật** — `ServiceForm`
(`src/app/modules/config/config-clinic/service/form/service-form.ts`) hiện là bản Signal Forms duy
nhất, thay hẳn bản Reactive Forms cũ (đã xoá), không còn route spike.

**Kết quả: KHẢ THI — tất cả AC hành vi đều PASS sau khi vá đủ gotcha (xem dưới):**

| AC | Nội dung | Kết quả |
|---|---|---|
| AC1 | `groupServiceMaterial` — dynamic required khi chọn material | ✅ PASS (verify UI thật: chọn material → 2 ô số đỏ ngay; bỏ chọn → hết đỏ) |
| AC2 | 6 field hoa hồng — `max(100)` khi percent | ✅ PASS (UI clamp qua `[max]` + validator tầng field đều chặn giá trị bypass qua console; đổi VND → hết giới hạn) |
| AC3 | FK required kể cả `0` (`emr_department_id`/`department_type`) | ✅ PASS (`null` và `0` đều invalid, kind `min` cho case `0`) |
| AC4 | `min_price`→`max_price` tự nâng, giữ khoảng giá thật | ✅ PASS (2 kịch bản đều đúng: max trống thì nâng theo min; có khoảng giá thật thì hạ min không ghi đè max) |
| AC5 | Cascade reset khi đổi `company_id`; derive `department_type` (lịch sử — cả hai field đã bỏ, D15/D16) | ✅ PASS (verify UI + console: unit/service_type/dept/room/byt_code → `null`, material → 1 dòng rỗng; chọn Khoa → derive đúng) |
| AC6 | `p-orderList` treatment step add/reorder/remove | ✅ PASS (DB thử nghiệm không có sẵn treatment step nào để test qua UI thật — verify qua simulate data trực tiếp trên component instance, mechanism đúng) |
| AC7 | Upload/xoá ảnh | ✅ PASS (sau khi vá — xem gotcha #1 dưới) |
| AC8 | `isSubmitting` disable Save; `AppConfirmDialog` khi dirty; lỗi hiện ngay khi mở | ✅ PASS cả 3 |
| AC9 | `effect()` chỉ tạo ở constructor, không rebuild `form()` | ✅ PASS (code review 2 pass QA) |
| AC10 | `tsc --noEmit` | ✅ PASS |
| AC12 | `service-form.ts`/`.html`/`service-list.ts`/`base-form.ts` không đổi | ✅ PASS (`git diff` xác nhận) |

## Gotcha thật gặp phải khi thực thi — BẮT BUỘC đọc trước khi viết form Signal Forms mới

1. **[CRITICAL] Field với giá trị `undefined` KHÔNG bind được, VĨNH VIỄN, dù key tồn tại trên object.**
   Đã verify thực nghiệm trực tiếp trên console browser: `rootField.x` trả về `undefined` (không phải
   `FieldTree`, không gọi `()` được) nếu giá trị HIỆN TẠI của field đó là `undefined` — **kể cả khi
   object CÓ key đó** (`'x' in model()` = `true` nhưng `model().x === undefined`). Một khi field "dính"
   trạng thái này, `.set()` lại giá trị THẬT (kể cả cùng object reference mới) mới phục hồi được.
   → **Model ban đầu (create mode) và mọi lần reset PHẢI seed TẤT CẢ field sẽ bind bằng giá trị CỤ THỂ
   KHÔNG PHẢI `undefined`** — dùng `null` cho scalar, `[]` cho mảng. Fix áp dụng:
   `emptyServiceModel()`/`emptyMaterial()` (`service-form.ts`) — mọi field seed `null`/`[]`,
   không field nào `undefined`. Đây là khác biệt CĂN BẢN so với Reactive Forms (`new FormControl(undefined)`
   hoạt động bình thường).
2. **`required()` coi số `0` là HỢP LỆ** — giống hệt gotcha `Validators.required` của Reactive Forms
   (`fe-patterns-gotchas-form.md`). Với field số bắt buộc, `null` vừa đúng ngữ nghĩa "rỗng" (invalid)
   vừa giữ field bind được (khác `undefined` ở gotcha #1) — KHÔNG dùng `0` làm placeholder rỗng cho
   field có `required()`.
3. **`FormField` (từ `@angular/forms/signals`) PHẢI tự thêm vào `imports: []`** của component standalone
   — thiếu thì `[formField]` không được nhận diện (`NG8002`). Dễ quên vì không có lỗi `tsc --noEmit`
   nào báo, và (xem gotcha #6) `ng build` lần đầu cũng có thể không báo.
4. **KHÔNG dùng `FieldTree<any>` làm type tham số** (vd `isFieldInvalid(field: FieldTree<any>)`) —
   TypeScript conditional type trong định nghĩa `FieldTree` bị "phân phối trên `any`"
   (`[any] extends [X] ? A : B` → hợp nhất `A | B` thay vì chọn 1 nhánh), gây `TS2345` khi nhận
   `FieldTree` cụ thể thật. Dùng generic `<V>(field: FieldTree<V>)` thay thế — tránh hẳn quirk này.
5. **[QUAN TRỌNG — quy trình] `ng build` MỘT LẦN không đủ tin cậy để verify template Signal Forms —
   phải test qua `ng serve` (dev server) thật.** Lần `ng build --configuration=development` đầu tiên
   chạy trong spike này **không hề báo lỗi** dù thiếu `FormField` import (gotcha #3) và dùng
   `FieldTree<any>` sai (gotcha #4) — cả 2 lỗi chỉ lộ ra khi chạy `ng serve` thật (dev server esbuild)
   và mở form trên browser. Nguyên nhân chưa xác định chắc chắn (nghi ngờ liên quan cache/pipeline khác
   nhau giữa 2 lệnh), nhưng bài học thực dụng: **form Signal Forms mới PHẢI mở thử trên `ng serve` thật
   trước khi coi là xong, không tin `ng build` một lần là đủ** — khác hẳn khuyến nghị cũ trong file này
   (mục 7 cũ) vốn coi `ng build` là đủ để bắt lỗi AOT template.
6. **PrimeNG suy luận generic type cho `[formField]` có thể lệch với field kiểu union** (`string|number`,
   `string|null`) — gặp ở `examination_specialty_code`/`point`, lỗi `TS2322` lúc `ng build`. Fix bằng
   `$any(rootField.x!)` — pattern bypass chuẩn đã dùng sẵn nơi khác trong codebase, không phải hack mới.
7. **Nghi vấn (chưa kết luận chắc)**: gõ phím qua tool automation trình duyệt (`computer.type`) không
   luôn khiến `<input pInputText>` thuần (native `[formField]` path) đồng bộ ngược vào field — value
   hiện đúng trên DOM nhưng `rootField.x().value()` vẫn `null`. Dispatch native `input` event thủ công
   qua `element.dispatchEvent(new Event('input', {bubbles:true}))` thì đồng bộ đúng ngay — nên nhiều khả
   năng là quirk của tool test tự động (không fire event `input` thật), KHÔNG phải lỗi `[formField]`/
   `BaseSignalForm`. Cùng thao tác gõ qua tool trên `p-inputnumber`/`p-select` (component PrimeNG thật,
   không phải `pInputText` — 1 directive trên input thuần) lại đồng bộ đúng bình thường. Story sau nếu
   test qua browser tự động, gặp input `pInputText` "không nhận giá trị gõ" → nghi ngờ tool trước, verify
   lại bằng thao tác người dùng thật hoặc dispatch event thủ công trước khi kết luận là bug.
8. **[HIGH] `modelService.getInitialObject()` dưới `BaseSignalForm` BẮT BUỘC trả object đủ key, giá
   trị rỗng dùng `null`/`[]`** — không được trả `{}` mặc định của `BaseApiService`. Lý do: nếu form
   dùng đường `ngOnInit()` mặc định của `BaseSignalForm` (không override), `model` khởi tạo rỗng `{}`
   trước khi có data — mọi `rootField.x!()` trong template gọi lên field không tồn tại → TypeError
   ngay khung hình render đầu (chưa kịp có response GET). `BaseSignalForm.ngOnInit()` đã tự seed
   `model` bằng `getInitialObject()` trước khi GET resolve (chặn crash) và merge đè khi có response
   (chặn field thiếu key do BE `omitempty`) — nhưng chỉ đúng nếu subclass override `getInitialObject()`
   trả đủ key (xem ví dụ `ServiceService.getInitialObject()`). Form override hẳn `ngOnInit()` (như
   `ServiceForm` tự làm qua `_applyModel()`) không phụ thuộc đường mặc định này, vẫn phải tự đảm bảo
   gotcha #1 khi seed model riêng.
9. **[CRITICAL] `<input pInputText type="text">` (KHÔNG phải `<textarea>`/`p-inputnumber`/`p-select`/
   custom control) coi field seed `null` là field SỐ — gõ chữ luôn lỗi `parse`, gõ số mất số 0 đầu.**
   Đã đọc thẳng source `node_modules/@angular/forms/fesm2022/signals.mjs` hàm `getNativeControlValue()`
   (~dòng 850-869): với `element.tagName === 'INPUT' && element.type === 'text'`, nếu giá trị HIỆN TẠI
   của field là `typeof === 'number'` HOẶC `null`, Signal Forms ép `Number(element.value)` mỗi lần gõ
   — gõ chữ → `NaN` → lỗi `NativeInputParseError` (kind `parse`), field giữ nguyên `null` VĨNH VIỄN dù
   DOM vẫn hiện ký tự đã gõ (không crash, không lỗi compile — âm thầm y hệt gotcha #1). Gõ số (vd SĐT
   `0912345678`) → parse thành `912345678`, MẤT SỐ 0 ĐẦU. Đây là hệ quả TRỰC TIẾP của quy ước "seed
   mọi field `null`" (gotcha #1) — quy ước đó đúng cho field số/reference nhưng SAI cho field text.
   → **Field kiểu `string` bind `<input pInputText>` (không phải `<textarea>`) PHẢI seed `''` (chuỗi
   rỗng), KHÔNG `null`** — `typeof '' === 'string'` nên tránh nhánh parse-số; `required()` vẫn coi `''`
   là invalid, không đổi ngữ nghĩa "rỗng". `<textarea pTextarea>` KHÔNG bị ảnh hưởng (`element.tagName`
   là `TEXTAREA`, khác nhánh code) — vẫn seed `null` bình thường. Áp dụng: mọi `.value.set(null)` cho
   field text `pInputText` (kể cả reset/clear thủ công trong code, không chỉ `emptyXModel()`) phải đổi
   thành `.set('')`. Đã fix `ServiceForm` (`name`/`code_old`/`emr_service_code`/material `note`) và
   `CustomerFormV2` (28 field) ngay khi phát hiện — kiểm tra lại MỌI form Signal Forms khác nếu có.
   Rất có thể chính là nguyên nhân thật của nghi vấn gotcha #7 phía trên (tool automation gõ vào field
   seed `null` trúng ngay nhánh parse-số này, không hẳn do tool không fire event).
10. **[HIGH] `required()` dùng `isEmpty()` nội bộ — KHÔNG coi mảng rỗng `[]` là "rỗng"**, khác hẳn
    `Validators.required` của Reactive Forms (coi mảng rỗng là invalid). Field multi-select bắt buộc chọn
    ≥1 giá trị (vd `serviceTypeId` dạng `number[]`; ví dụ gốc `companyId` đã bỏ, D15) nếu dùng thẳng `required(field)` sẽ ÂM
    THẦM đổi hành vi — submit được dù chưa chọn gì, không lỗi compile, không cảnh báo runtime. Phát hiện ở
    epic `config-signal-forms` story-03 (`CustomerCareTypeForm`). Fix: viết `validate()` thủ công cho field
    mảng bắt buộc, dùng `requiredError()` (export sẵn từ `@angular/forms/signals`) khi
    `value()?.length === 0`:
    ```ts
    validate(p.serviceTypeId, ({value}) => (value()?.length ?? 0) === 0 ? requiredError() : null);
    ```
11. **[HIGH] `[formField]` KHÔNG được đặt `maxlength`/`readonly`/`min` (HTML attribute thô) trên cùng
    element — lỗi `NG8022`, chỉ lộ ra ở `ng build`/`ng serve` (AOT template check), KHÔNG lỗi `tsc --noEmit`.
    Nguyên nhân: Signal Forms tự gắn các attribute này từ metadata validator (`maxLength()` trong
    `buildSchema()`) — đặt tay lần 2 bị coi là xung đột. Fix: xoá hẳn attribute HTML thô khỏi template;
    nếu cần giữ hành vi giới hạn ký tự/readonly, khai qua `maxLength(field, N)`/`disabled(field, {when})`
    trong `buildSchema()`, KHÔNG viết trực tiếp trên thẻ. Phát hiện ở epic `config-signal-forms` khi chạy
    `ng build` thật lần đầu cho toàn bộ 112 form — 49/120 lỗi AOT thuộc nhóm này, rải khắp `config-emr`.
    `maxLength()`/`.length` đếm **UTF-16** (emoji, ký tự ngoài BMP = 2) còn BE Go giới hạn bằng `utf8.RuneCountInString`
    (rune) ⇒ cần khớp giới hạn BE chính xác thì đếm code point `[...s].length` bằng validator tự viết.
    Lưu ý: `[min]`/`[max]` khai qua Input PROPERTY của component PrimeNG (vd `p-inputnumber [min]="0"`,
    binding Angular `[min]`, KHÔNG phải attribute HTML thô) AN TOÀN, không dính gotcha này — đã verify
    dùng lại nguyên trạng ở nhiều form (`marketing-signal-forms` epic) không lỗi `ng build`.
    Bản `@angular/forms/signals` hiện tại **CÓ** `readonly(path, configOrLogic)` (`signals.mjs`) — ghi
    chú cũ "chỉ có `disabled()`" đã lỗi thời. Lưu ý kiểu: `maxLength()`/`minLength()` chỉ nhận path
    **non-null**, field `string | null` phải cast (`maxLength(p.x! as any, N)`).
    ⚠ Dạng **property binding** `[readonly]="expr"`/`[disabled]="expr"` trên control có `[formField]` là ca
    KHÁC và NGUY HIỂM HƠN: không lỗi build, nhưng directive `FormField` đẩy ngược field-state xuống input
    (`FIELD_STATE_KEY_TO_CONTROL_BINDING` + `setInputOnDirectives` trong `@angular/forms/fesm2022/signals.mjs`)
    ⇒ giá trị vừa bind bị ghi đè về `false`, ô VẪN sửa được. Khoá thật chỉ có một đường: khai trong
    `buildSchema()`. `tsc` lẫn `ngc` đều xanh ⇒ chỉ smoke hoặc test đọc trạng thái ô mới bắt được (epic
    `treatment-estimate` story-08: 3 ô giữ giá gõ được dù đã `[readonly]`, QA đọc code kết luận nhầm "đã khoá").
12. **[QUAN TRỌNG — quy trình, củng cố gotcha #5]** `tsc --noEmit` sạch cho TỪNG file KHÔNG đảm bảo
    `ng build` sạch cho TOÀN APP. Epic `config-signal-forms` (112 form, 29 story) mỗi story đều báo
    `tsc --noEmit` sạch, nhưng khi chạy `ng build --configuration=development` một lần cho TOÀN BỘ sau khi
    merge hết mới lộ ra 120 lỗi AOT thật (gotcha #6 tái diễn ở hàng chục file, `NG8022` ở gotcha #11, và
    vài field model sai kiểu từ Reactive Forms cũ — `boolean` lẽ ra `number`, `number` lẽ ra `string` —
    do `model: any` trước đây che giấu, Signal Forms strict-type mới moi ra). Nguyên nhân: mỗi story chỉ
    tự kiểm phạm vi file mình sửa, không ai chạy `ng build` NGUYÊN APP (không có trình duyệt trong môi
    trường agent). Bài học: sau khi fan-out nhiều story cùng đổi sang Signal Forms, **bắt buộc 1 lần chạy
    `ng build` toàn app cuối đợt** trước khi coi xong, không chỉ tin `tsc` từng story cộng lại. `@if
    (rootField) { ... }` bọc ngoài toàn template cũng là lỗi thật (`TS2774`, "condition always true") — xoá
    hẳn wrapper này, `rootField` luôn tồn tại từ field initializer của `BaseSignalForm`, không cần guard.
    KHÔNG dùng `required()` cho field kiểu mảng nếu ngữ nghĩa gốc là "phải chọn ít nhất 1".
13. **[CRITICAL] `dialogConfig.data.model` là object RỖNG `{}` (không phải thiếu key `model`) làm form
    "Tạo mới" vỡ VĨNH VIỄN — `BaseSignalForm` coi `{}` là "đã có model từ dialog" (truthy check), bỏ qua
    hẳn nhánh seed `getInitialObject()` cho create-mode, `model` kẹt `{}` mãi mãi → mọi field vỡ theo
    gotcha #1. Nguyên nhân: 1 số `list.ts` cũ dùng chung 1 hàm `openForm(model)` cho cả create/edit,
    gọi `openForm({})` cho create thay vì gọi `openForm()`/omit hẳn key `model` — vô hại dưới Reactive
    Forms cũ (`new FormControl(undefined)` chịu được) nên chưa từng lộ ra, chỉ vỡ khi form đã chuyển sang
    Signal Forms. Phát hiện ở epic `config-signal-forms` story-11 (QA) — lan tới **7 file `list.ts`**
    trong `config-emr` dùng đúng pattern này, không phải ca đơn lẻ. Đã fix TẠI GỐC ở
    `base-signal-form.ts` (không vá từng `list.ts` — không phòng được form mới lặp lại pattern):
    coi `model` là "không có" khi `!model || Object.keys(model).length === 0`, áp dụng ĐỒNG BỘ ở cả
    constructor (đọc `dialogConfig.data.model` lần đầu) VÀ nhánh create-mode của `ngOnInit()` (2 nơi đọc
    cùng nguồn, sửa thiếu 1 chỗ vẫn vỡ). An toàn cho form khác vì chỉ đổi hành vi khi model rỗng thật sự.
14. **[CRITICAL] `dialogConfig.data.model` CÓ key (không rỗng) nhưng vẫn THIẾU key riêng lẻ nếu BE dùng
    `omitempty` cho field đang ở giá trị rỗng** (ví dụ gốc — field đã bỏ, D15: Go `CompanyID *uint json:"company_id,omitempty"`, NULL
    = "dùng chung toàn hệ thống" — một giá trị hợp lệ, không phải lỗi dữ liệu) — key biến mất HẲN khỏi
    JSON response, không phải `null`. Đường mặc định `BaseSignalForm.ngOnInit()` (trước fix) chỉ backfill
    `getInitialObject()` cho 2 nhánh: GET-theo-id và "không có model" — nhánh "CÓ model từ dialog" (phổ
    biến nhất, mọi form mở qua `updateRecord()` với `data:{model}` từ list) hoàn toàn BỎ QUA backfill →
    field bị `omitempty` vỡ vĩnh viễn theo gotcha #1 ngay khi mở sửa 1 record có field đó rỗng. Không lộ
    ra ở Reactive Forms cũ (`FormControl(undefined)` chịu được). Constructor KHÔNG gọi được
    `getInitialObject()` (service injected qua field subclass, chưa gán xong lúc `super()` của
    `BaseSignalForm` chạy) — fix bắt buộc đặt ở `ngOnInit()`, thêm nhánh `else` thứ 3: khi `data.model` có
    key thật, vẫn merge `{ ...getInitialObject(), ...model() }` để backfill field bị `omitempty`, giá trị
    thật LUÔN ghi đè default (thứ tự spread) nên an toàn cho form đã đủ key. Phát hiện ở epic
    `config-signal-forms` story-26 (`AiPromptForm.company_id` — đã bỏ, D15), nhưng RỦI RO ÁP DỤNG CHO MỌI FORM có field
    nullable+`omitempty` ở BE và không tự override `ngOnInit()` — verify lại `ng build`/`tsc` toàn app sau
    khi sửa vì đây là thay đổi ảnh hưởng gần như mọi form Signal Forms dùng đường mặc định.
> **Gotcha #15–#55 nằm ở `signal-forms-gotchas-2.md`** (tách 2026-08-12 vì file vượt 350 dòng).

## Pattern mẫu khi tạo form Signal Forms mới

Xem `src/app/modules/config/config-clinic/service/form/service-form.ts`/`.html` (production, đầy đủ
nhất repo hiện tại) làm khuôn: `emptyXModel()` (seed toàn `null`/`[]` cho số/reference, `''` cho text
`pInputText` — gotcha #1 + #9), `effectOnChange()`
helper (skip lần chạy đầu, tránh cascade-reset đè dữ liệu vừa load ở edit mode), `buildSchema()` chỉ
dùng import/hằng số module-level (injected service dùng trong `LogicFn`/`effect()` closure), `applyEach()`
cho field mảng dạng bảng (`groupServiceMaterial`), override `onSubmit()`/`getPayload()` khi có field
ngoài `FieldTree` (vd file upload) hoặc BE cần lọc/transform payload riêng.

Form đơn giản (không FormArray/upload) soi mẫu gọn hơn: `src/app/modules/hr/allowance/form/allowance-form.ts`.

### Hook `prepareModel` (base)

`BaseSignalForm.ngOnInit` gọi `this.model.set(this.prepareModel(x))` ở CẢ 3 nhánh (edit GET — cả set
khởi tạo lẫn sau subscribe; create trống; create có prefill `data.model`). `prepareModel` mặc định no-op
— override khi form cần chuẩn hoá model trước khi bind (spec giữ hành vi: `base-signal-form.spec.ts`).
- Form ĐÃ có `ngOnInit`/`_applyModel` riêng → chuẩn hoá ở **choke point** mà MỌI đường (create trống /
  create có prefill / edit) đi qua ngay trước `model.set`, KHÔNG dùng cả hai; ⛔ KHÔNG chép 3 nhánh
  `ngOnInit` của base vào form. Bẫy đã gặp: đặt ở nhánh `else` create-trống ⇒ đường create-có-prefill
  bỏ qua.
- Scope chi nhánh: `trackScopeFields('branch_id')` (chỉ còn 1 tham số branch) set `currentBranchId`;
  bind signal `number | undefined` vào input `number | null` → `?? null`, chỉ `ngc` bắt được (B.11 hard-rules).

> ⚠ **Lịch sử (D15, story-61 xoá khỏi `base-signal-form.ts`)**: khuôn company trong base — helper `seedCompanyId`, `currentCompanyId`, "record dùng chung" (`company_id` NULL = áp dụng mọi thương hiệu) với `watchSharedRecordOffer`/`offerAssignSharedRecord`/`guardCompanyBeforeSubmit`/`sharedOfferDone`, spec quét `required\(p\.company_id` — KHÔNG còn tồn tại. Đừng dựng lại; form mới không có field company. Bài học chuyển được: muốn hỏi user điều gì về record thì hỏi LÚC MỞ form, đừng `model.update` lúc submit (control con reload/reset trước khi payload dựng async ⇒ mất dữ liệu im lặng).

## Form NHÚNG (không phải dialog) + thay `emitEvent: false`

Component nhúng (tab/khối trong 1 trang, nhận `@Input`, không có `dialogRef`/`modelService`) KHÔNG
dùng `BaseSignalForm` — có khuôn riêng, kèm cách thay `{ emitEvent: false }` (Signal Forms không có).
→ **`frontend/skill/skills/building-angular-signal-forms/SKILL.md`**

## Khuyến nghị

Signal Forms **khả thi kỹ thuật hoàn toàn** cho form Cluvix, kể cả form phức tạp nhất repo — nhưng
gotcha #1 (giá trị `undefined`) là bẫy nghiêm trọng, âm thầm (không lỗi compile, chỉ vỡ UI runtime) mà
**bất kỳ ai viết form Signal Forms mới cũng sẽ dính nếu không biết trước**. `BaseSignalForm` đã tự bảo
vệ đường `ngOnInit()` mặc định (gotcha #8) — form override `ngOnInit()` riêng (đa số form phức tạp) vẫn
tự chịu trách nhiệm seed model đủ field non-`undefined`. Quyết định rollout diện rộng cho các form còn
lại **KHÔNG nằm trong kết luận này** — đó là quyết định riêng của user (xem
`bmad/signal-forms-poc/prd.md` mục "Rủi ro nghiệp vụ").


> Tách từ `skill/skills/building-angular-signal-forms/SKILL.md` (2026-08-12, file gốc vượt 350 dòng). Đọc `skill/skills/building-angular-signal-forms/SKILL.md` TRƯỚC
> (gotcha #1–#49 nền, gồm cả `signal-forms-gotchas-2.md`), file này chỉ bổ sung phần dành riêng cho
> form nhúng.

## Khuôn form NHÚNG (không phải dialog) — `form()` trực tiếp, KHÔNG `BaseSignalForm`

`BaseSignalForm` gắn với dialog/CRUD: nó bắt buộc `abstract modelService: BaseApiService<T>`, có
`onSubmit()` create/update và `closeForm()` → `dialogRef.close()`. **Component nhúng** (tab/khối trong
1 trang, nhận dữ liệu qua `@Input`, tự gọi service riêng) KHÔNG có thứ nào trong số đó → dùng `form()`
trực tiếp. Đúc từ epic `emr-signal-forms-examination` (5 component trong `emr/examination/`).

```ts
interface XxxFormModel { a?: string | null; b?: number | null; }
function emptyXxxModel(): XxxFormModel { return { a: null, b: null }; }   // ĐỦ key, không undefined

@Component({ imports: [/* … */ FormField], changeDetection: ChangeDetectionStrategy.OnPush })
export class Xxx {
  readonly appointment = input.required<ReceptionModel>();   // @Input -> input(): cha KHÔNG phải sửa
  readonly isLocked = input(false);

  private readonly model = signal<XxxFormModel>(emptyXxxModel());
  protected readonly rootField = form(this.model, (p) => this.buildSchema(p));  // field initializer

  protected isFieldInvalid<V>(field: FieldTree<V>): boolean { return field().invalid(); }

  private buildSchema(p: SchemaPathTree<XxxFormModel>): void {
    required(p.a!);
    disabled(p, { when: () => this.isLocked() });   // thay form.disable()
  }
}
```
Đọc `this.model()` (cả cụm) / `this.rootField.x!().value()` (1 field); ghi `this.model.update(m => ({...m, x}))`.
Template: bỏ `[formGroup]`, `formControlName="x"` → `[formField]="$any(rootField.x!)"` (**mặc định bọc
`$any()`** — field union `string | null` gần như luôn lệch generic PrimeNG, gotcha #6, chỉ `ng build`
mới bắt), thêm `FormField` vào `imports`.

**Khoá ĐỘNG** (`[formControlName]="item.key"` trong `@for`, hoặc `formGroupName` + khoá tính được):
thêm helper trả field con, bind `[formField]="$any(fieldOf(item.key))"`:
```ts
protected fieldOf(key: string): any { return (this.rootField.nested as any)?.[key]; }
```
Object lồng phải seed ĐỦ MỌI khoá ngay từ `emptyXxxModel()` (thiếu khoá nào là ô đó vỡ vĩnh viễn).

**`@Input` → `input()`**: xoá luôn bug class "@Input chỉ set trong `ngOnInit` bị stale khi route-reuse"
(`frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-signals.md`). Thay `ngOnChanges` bằng `effect()` — nhưng **giữ ĐÚNG
những gì bản cũ đồng bộ**: nếu `ngOnChanges` cũ chỉ sync 1 field (vd `version`) và cố ý giữ nguyên field
user đang gõ dở, thì effect mới cũng chỉ được sync đúng field đó, KHÔNG seed lại cả model.
Sửa template bằng regex `\bisLocked\b` → `isLocked()` sẽ **làm hỏng TÊN input trong binding**
(`[departmentType]=` → `[departmentType()]=`) — grep lại `"()]="` sau khi thay.

**Nhúng form con có `form()` riêng vào dialog `BaseSignalForm`** (vd `ExamTemplateContentForm` trong
`ExamTemplateForm`): `closeForm()` của base chỉ xét `rootField().dirty()` của CHA ⇒ bấm Huỷ mất sạch
nội dung form con không hỏi. Con lộ `hasChanges()` (so giá trị payload với bản seed, gotcha #54); cha
`override closeForm(e) { if (this.child?.hasChanges()) this.rootField().markAsDirty(); super.closeForm(e); }`.
Mở dialog truyền `dismissableMask: false, closeOnEscape: false, closable: false` (3 đường đóng này
bỏ qua `closeForm()`); nút Back trình duyệt vẫn đóng thẳng (`DialogWrapperService.bindBackButton`).

## Thay `{ emitEvent: false }` — KHÔNG có tương đương ở Signal Forms

Reactive Forms dùng `patchValue(x, { emitEvent: false })` để ghi field kỹ thuật (thường là `version`
sau khi lưu) mà KHÔNG bật cờ "có thay đổi chưa lưu". Signal Forms ghi signal là **luôn** notify → bắt
sự kiện thay đổi sẽ tự bật lại cờ sau mỗi lần lưu (gọi API thừa, hoặc vòng lặp nếu có autosave).

Giải: đảo chiều — so **chữ ký của tập field NGHIỆP VỤ** (loại `version`/`id`) giữa hiện tại và lần lưu
gần nhất. Helper dùng chung: `share/utils/signal-form-pending.ts` → `createPendingTracker(model, businessKeys)`
trả `{ hasPendingChanges, markSaved }`. Gọi `markSaved()` sau khi seed xong và sau MỖI lần lưu thành công.
`businessKeys` phải cố định thứ tự (chữ ký so bằng `JSON.stringify`).
