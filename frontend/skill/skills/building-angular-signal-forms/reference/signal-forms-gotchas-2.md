# Signal Forms gotchas #15-55

## Mục lục


---


> Tách từ `skill/skills/building-angular-signal-forms/SKILL.md` (2026-08-12, file gốc vượt 350 dòng). Đánh số TIẾP TỤC từ file gốc —
> mọi tham chiếu dạng "gotcha #N" trong `skill/`/BMAD vẫn đúng, chỉ đổi chỗ đọc.
> Gotcha #1–#14 + kết luận spike + pattern mẫu: `skill/skills/building-angular-signal-forms/SKILL.md`.
> Form nhúng (không dialog) + thay `emitEvent: false`: `frontend/skill/skills/building-angular-signal-forms/SKILL.md`.

15. Khi convert `ngOnInit()` từ Reactive Forms (thường LUÔN gọi `GET(id)` khi Sửa) sang dùng thẳng
    `dialogConfig.data.model` (record từ list) — kiểm BE trước: list-API và get-by-id-API có SELECT/
    Preload cột khác nhau không (rất phổ biến — list chỉ trả cột tóm tắt). Nếu có, PHẢI giữ nhánh
    `GET(id)` khi Sửa, không được bỏ chỉ vì `dialogConfig.data.model` "trông có vẻ đủ dữ liệu" (phát hiện
    ở epic `config-signal-forms` story-29 — `PsychQuizForm` mất hết câu hỏi khi Sửa vì BE `ListQuiz`
    không `Preload("Questions.Options")`, chỉ `GetQuiz` mới có).
16. `toObservable(computed(() => field.value()))` + `filter(index > 0)` chỉ chặn được lần seed ĐỒNG BỘ
    (constructor) — nếu có 1 hàm khác (vd load ban đầu trong `ngOnInit()`, bất đồng bộ) cũng ghi field
    đó, lần ghi đó sẽ lọt qua filter và bị hiểu nhầm là "user đổi", gây gọi API thừa/ghi đè dữ liệu vừa
    nhập. Fix bền vững: so sánh giá trị mới với 1 signal PHỤ được ghi ĐỒNG BỘ cùng lúc bởi chính hàm load
    đó, không dùng timing/index (phát hiện ở story-27 — `CallCenterExtensions`).
17. Field cố ý để ngoài `FieldTree` (như `fileImage`, hoặc field text lớn dạng JSON) KHÔNG tự đồng bộ
    theo `model()` — nếu form override `ngOnInit()` hoàn toàn, PHẢI tự seed field đó từ `model().x` (1
    lần, guard tránh ghi đè lúc user đang gõ dở nếu `model()` có thể đổi reference vì lý do khác), nếu
    không field sẽ mãi rỗng khi Sửa và Lưu sẽ xoá mất dữ liệu thật ở field đó (phát hiện ở story-22 —
    `CronJobForm.paramsText`).
18. **[CRITICAL] Form có BẢNG dòng + recalc: `p-table` PHẢI khai `[rowTrackBy]` theo khoá ổn định.**
    Mặc định PrimeNG track theo THAM CHIẾU object (`rowTrackBy = (index, item) => item ?? index`,
    `primeng-table.mjs`). Recalc kiểu Signal Forms (`model.update(m => ({...m, rows: rows.map(...)}))`)
    tạo object dòng MỚI sau MỖI phím gõ → toàn bộ `<tr>` bị huỷ/dựng lại → **user đang gõ dở trong ô
    `p-inputnumber` mất focus sau mỗi ký tự**. Không lỗi `tsc`, không lỗi AOT, chỉ lộ khi gõ tay nhiều
    ký tự liên tiếp trên browser thật. Fix: thêm field FE-only `_uid` (counter tăng dần) lúc tạo dòng +
    `[rowTrackBy]="rowTrackBy"` với `(i, row) => row?._uid ?? i`, và loại `_uid` khỏi payload ở
    `getPayload()`. KHÔNG dùng `id` làm khoá — dòng mới chưa lưu có `id = null`, nhiều dòng mới trùng
    khoá. Kèm theo: trong hàm recalc chỉ tạo object mới cho dòng THỰC SỰ đổi số (so field tính ra với
    field cũ, bằng nhau thì `return row`), và bỏ luôn `model.update()` khi không có gì đổi.
    (Áp dụng `OrderPlanForm` 2026-08-09, `ServiceComboForm` 2026-08-11 — form thứ 2 này đã ở production
    và dính đúng bẫy: `dataKey="id"` trong khi row type KHÔNG có field `id` nên hoàn toàn no-op. Mọi form
    Signal Forms có bảng dòng + recalc đều phải kiểm mục này.)
    → Có `_uid` rồi thì **đừng dừng ở `trackBy`**: mọi side-state per-row cũng phải key theo nó — xem **#51**.
19. **[CRITICAL] `undefined` → `null` KHÔNG vô hại về phía BE: nhiều handler merge theo KEY PRESENCE.**
    Reactive Forms để field chưa nhập là `undefined` và `JSON.stringify` **bỏ hẳn key**; Signal Forms
    CẤM `undefined` (gotcha #1) nên buộc phải gửi `null` → **key xuất hiện trong JSON**. Handler Go kiểu
    `if v, ok := data["x"]; ok {...}` (merge map) hoặc `if src.X != nil {...}` (merge struct) hiểu
    "key vắng = giữ nguyên giá trị DB", còn `null` = **ghi đè**. Hệ quả: convert form sang Signal Forms
    có thể âm thầm ghi đè dữ liệu DB bằng `0`/rỗng ở đúng những field user không hề chạm.
    → Khi convert form có PUT/update: đọc handler BE, liệt kê field merge-by-presence, rồi đối chiếu
    TẬP KEY của `getPayload()` mới với `rfModel.getRawValue()` cũ; field nào cũ ra `undefined` mà mới ra
    `null` thì quy đổi ngược trong `getPayload()`. Ca thật (`OrderPlanForm` 2026-08-10): `applied_position`
    `[]` ghi đè NULL, `discount_other` cấp đơn `NaN`→`null` (giữ DB) thành `0` (ghi đè).
    Nhớ kèm: `extractValue()` mặc định duyệt TOÀN BỘ key trên `model()` (gồm cả nested BE trả về) —
    form nào spread response BE vào model BẮT BUỘC override `getPayload()` whitelist tường minh.
    Không chỉ là ghi đè `null`: key **field quan hệ** (mảng con BE trả về, vd `serviceMaterial`) lọt
    payload làm BE `Updates(map)` dùng key làm TÊN CỘT → `Error 1054`, hỏng hẳn thao tác lưu.
20. **Validator có metadata (`min`/`max`/`maxLength`) tự đẩy xuống INPUT của component, đổi UX.**
    `min(path, 1)` đăng ký `MIN_NUMBER` → Signal Forms gọi `setInputOnDirectives('min', 1)`
    (`signals.mjs`) → set thẳng `@Input() min` của `p-inputnumber` (kế thừa `BaseInput.min`) → ô số
    **CLAMP** giá trị user gõ thay vì cho nhập rồi báo đỏ như Reactive Forms. Muốn giữ đúng UX cũ: dùng
    `validate(path, ...)` trả `minError(1)`/`maxError(n)` — cùng error kind cho `getError()`, không sinh
    metadata. (Đây cũng là mặt kia của gotcha #11: chính cơ chế đó gây `NG8022` khi đặt tay attribute
    trùng trên element native.)
21. **1 `effect()` cho cả mảng KHÔNG tương đương N listener per-control.** Bản Reactive Forms gắn
    `valueChanges` vào TỪNG control nên chỉ dòng vừa đổi bị xử lý; gom thành 1 `effectOnChange` đọc cả
    mảng thì action chạy trên MỌI dòng → ghi đè giá trị đã lưu ở DB của dòng không liên quan (ca thật:
    `numTreatment` tính lại theo `base × quantity` cho mọi dòng, trong khi bản gốc ưu tiên giá trị lưu
    khi định nghĩa dịch vụ đã đổi sau lúc lên đơn). Fix: giữ `Map<_uid, chữ-ký-field-điều-khiển>`, trong
    action chỉ xử lý dòng có chữ ký khác lần trước; seed chữ ký ngay tại chỗ TẠO dòng và tại chỗ đã tự
    tính giá trị tường minh (nếu không, lần đổi đầu tiên sẽ bị bỏ qua hoặc bị tính đè).
22. **[HIGH] `handleSelectedDate()`/`formatDate()` (`base-form.ts`) CHỈ có ở `BaseForm`, KHÔNG có ở
    `BaseApp`** (base thật của `BaseSignalForm`) — form Signal Forms KHÔNG gọi được 2 hàm này. Nghiêm
    trọng hơn: `handleSelectedDate()` tự side-effect `patchValue()` bên trong bằng cách đọc attribute DOM
    `formControlName` — cơ chế không tồn tại ở Signal Forms (`[formField]` không set attribute này).
    Field date nhập tay qua Enter (`(keydown.enter)="handleKeyPressXxx($event)"`, pattern rất phổ biến
    trong `marketing/`, `crm/`...) → viết lại logic PARSE THUẦN (không side-effect) thành hàm
    module-level riêng trong file form mới (theo convention `validateMonthFormat()` ở
    `allowance-form.ts`), rồi set qua `rootField.x!().value.set(date)`. Đã lặp lại **2 lần** (`CustomerFormV2`
    — `signal-form-customer-appointment`; `KpiForm`/`DataFacebookForm` — `marketing-signal-forms`) — coi
    là gotcha chuẩn, sẽ còn gặp ở mọi form Signal Forms có field date nhập tay tương tự.
23. Mất nhánh "dùng `model` đã pre-fetch từ list (list tự `GET(id)` trước khi mở dialog), skip GET lại
    trong form" khi convert `ngOnInit()` sang đường mặc định của `BaseSignalForm` (LUÔN `GET(id)` khi có
    `id()`) — **tưởng là regression (double network call) nhưng KHÔNG PHẢI**:
    `app/core/interceptors/api-dedupe.interceptor.ts` (micro-cache GET, TTL mặc định 5000ms, áp toàn app
    cho mọi request `/api/`) hấp thụ hoàn toàn request thừa (2 GET cùng URL trong vòng vài chục ms).
    Verify bằng `read_network_requests` (Chrome MCP) thật trước khi coi đây là bug cần fix — đừng chỉ suy
    diễn từ đọc code tĩnh (phát hiện ở QA `story-02-data-facebook-form`, epic `marketing-signal-forms`).
24. **[HIGH] Nới kiểu field trong model để phục vụ form → vỡ AOT ở component KHÁC dùng chung model.**
    Field bind `p-datepicker` phải nới `number` → `number | Date | null`; component khác nhân thẳng
    `field * 1000` trong TEMPLATE sẽ lỗi `TS2362` — `tsc --noEmit` KHÔNG bắt (template chỉ type-check ở
    AOT). → Trước khi nới kiểu bất kỳ field model nào: `grep` mọi consumer (`detail-*`, `*-list`,
    `*-logs`, export) trong cả `.ts` LẪN `.html`. Sửa bằng getter chuẩn hoá đủ mọi nhánh ở component
    consumer, KHÔNG `$any()` (che mất kiểu thật). Symbol nới kiểu mà 0 consumer ngoài form → an toàn,
    không cần build lại (`StockTransferModel.lines/.version`).

25. **[CRITICAL] Seed `model` PHẢI chạy TRƯỚC mọi nhánh `return` sớm trong `ngOnInit()`** (check quyền,
    thiếu data...). Nhánh return sớm VẪN để component render 1 khung hình — `dialogRef.close()` của
    DynamicDialog không huỷ view đồng bộ — nên model còn rỗng `{}` thì mọi `rootField.x!()` trong
    template ném TypeError (gotcha #1). Bản Reactive Forms được `@if (rfModel && ...)` che cho vì
    `rfModel` chỉ gán trong `initialForm()`; **xoá `rfModel` khỏi guard `@if` lúc convert chính là xoá
    lá chắn đó**. `tsc --noEmit` lẫn `ng build` đều KHÔNG bắt được (lỗi runtime trên nhánh điều kiện).
    Thứ tự đúng: seed model → check/return → load bất đồng bộ (phát hiện ở QA epic `emr-signal-forms`,
    2 form ký `form-sign-medical-record`/`form-close-episode`).
26. **`Validators.requiredTrue` KHÔNG có tương đương** trong `@angular/forms/signals`. Checkbox bắt buộc
    tích (xác nhận trước khi ký, đồng ý điều khoản...) phải viết tay — `required()` coi `false` là "có
    giá trị" nên cho submit khi chưa tích, không lỗi compile, không cảnh báo:
    ```ts
    validate(p.confirm!, ({ value }) => (value() === true ? null : requiredError()));
    ```
27. **Validator cấp `fb.group` (cross-field) → `validate()` ở path GỐC.** Signal Forms không có khái niệm
    group validator; khai `validate(p, ({value}) => ...)` trên chính `p` và trả `{kind: 'tên-tự-đặt'}`,
    rồi `onSubmit()` đọc `rootField().errors()` lọc theo `kind` để bắn đúng thông báo cũ. Lưu ý `kind`
    tự chế rơi vào nhánh mặc định `common.validation.invalid` của `getError()` — chỉ dùng cách này khi
    lỗi vốn hiển thị bằng **toast**, không phải text dưới field (áp dụng `CheckinReception`:
    `dateRangeValidator`/`inRangeValidator` → 2 `validate(p, ...)` giữ `kind` `dateRange`/`outOfRange`).

28. **[HIGH] Field khai kiểu `| null` KHÔNG bind được `[formField]` trên `<input pInputText>` — lỗi chỉ
    lộ ở `ng build`.** `TS2322: Type 'WritableSignal<string | null>' is not assignable to type
    '{ (): string; set: (v: string) => void; } | { (): number | null; ... }'`. `tsc --noEmit` hoàn toàn
    im lặng. Đây là mặt kia của gotcha #9: đã bắt buộc seed `''` cho field text thì **kiểu cũng phải là
    `string`**, đừng khai `string | null` cho "an toàn".
    ⚠ Phân biệt rõ: **optional (`x?: string`) thì KHÔNG sao** — verify bằng `patients-form.html`
    (`rootField.fullname!` với `fullname?: string`) build sạch; chỉ `| null` tường minh mới vỡ.
    Phát hiện ở epic `loyalty-signal-forms` story-03 (`BrokerPaymentInfoRow.label/value`); cùng lỗi này
    cũng đang làm hỏng `med-stock-check`/`med-stock-export` (`[formField]="rootField.code!"`) của epic
    `med-signal-forms`.
29. **`p-autocomplete` suy generic `[formField]` thành `number | null | undefined`** — field giữ object
    hiển thị (`{value, label}`) sẽ `TS2322` lúc `ng build` (biến thể của gotcha #6). Fix bằng
    `$any(rootField.x!)`. (`loyalty-signal-forms` story-06 — `BrokerForm.customer_data`.)
30. **`toFormData()` (`share/services/utils.service.ts`) biến `null`/`undefined` thành `''`** — nên với
    form gửi **multipart**, "bỏ key" và "gửi null" cho kết quả GIỐNG NHAU ở phía BE, và bản Reactive
    Forms cũ **cũng đã luôn gửi `''`** chứ không phải "key vắng". ⇒ Với form multipart, cách an toàn
    nhất khi convert là **giữ đúng tập key cũ**, đừng suy luận theo mô hình merge-by-presence của JSON
    (gotcha #19 chỉ áp cho đường JSON). Ngược lại, key **thừa** rất nguy hiểm: field ảnh (`imageCoupon`)
    gửi rỗng = **xoá ảnh trong DB**. (`loyalty-signal-forms` story-04/05.)
31. **BE có thể ghi đè VÔ ĐIỀU KIỆN, lúc đó bỏ key KHÔNG cứu được.** Gotcha #19 giả định handler kiểu
    `if v, ok := data["x"]; ok` (key vắng = giữ DB). Nhưng handler kiểu "load record → gán thẳng
    `dst.X = src.X` → `Save()`" (vd `applyCouponUpdate`, `applyBrokerUpdate` của `loyalty`) thì **key
    vắng = gán zero-value = vẫn ghi đè**. ⇒ Khi audit payload phải phân **từng field** vào 2 nhóm:
    *ghi đè vô điều kiện* (bắt buộc gửi đúng giá trị đang hiển thị, lấy từ `model()`) và
    *merge-by-presence* (rỗng thì bỏ hẳn key). Đọc source hàm apply/Updates, đừng suy từ tên handler.

32. **[CRITICAL — lớp bug đắt nhất, epic `med-signal-forms` gặp 5 lần liên tiếp] Form mở Sửa từ
    `dialogConfig.data.model` (record của list) + BE `SaveDetails` delete-all-recreate = MẤT DỮ LIỆU
    ÂM THẦM.** Không phải lỗi Signal Forms, nhưng convert form là lúc DUY NHẤT phát hiện ra, và bản
    Signal Forms làm nó nặng hơn (seed mảng `[]` bắt buộc theo gotcha #1 → gửi `[]` = BE xoá sạch dòng).
    Cơ chế: list-API SELECT ít cột hơn get-by-id → `data.model` thiếu cột → form Sửa dựng payload từ dữ
    liệu thiếu → BE xoá dòng cũ rồi tạo lại theo payload thiếu. Không lỗi compile, không cảnh báo, chỉ
    mất dữ liệu. 5 ca THẬT tìm được trong 1 epic (đều tồn tại sẵn ở bản Reactive Forms trên production):
    `drug_recall` (list không Preload chi tiết → chọn thêm 1 lô là mất hết lô cũ) · `purchase_order`
    (SELECT thiếu `unit_id`) · `stock_import` (header thiếu `attachments` → **mỗi lần Lưu xoá sạch file
    đính kèm**) · `stock_export`/`stock_check` (thiếu `batch_id` → duyệt phiếu KHÔNG trừ kho, sai tồn
    thật) · `customer_prescription` (dòng thiếu TOÀN BỘ nhóm liều dùng + thang YHCT → sửa đơn là xoá
    sạch liều dùng). → **Quy trình bắt buộc khi convert form có mảng con**: (a) đọc handler list và so
    **TỪNG CỘT** của SELECT với entity — cả cột header lẫn cột dòng, "list có trả mảng chi tiết" KHÔNG
    đủ; (b) thiếu cột → thêm nhánh `GET(id)` khi Sửa (gọi GET trùng vô hại, gotcha #23); (c) `getPayload()`
    bỏ HẲN key mảng con khi đang Sửa mà mảng rỗng.
33. **`BaseApiService.afterLoad()`/`beforeSave()` chỉ convert `Date`↔unix cho field TOP-LEVEL** khai trong
    `unixTimesTampFields` (dot-path, không duyệt mảng). ⇒ Field ngày ở HEADER: không cần hàm parse thủ
    công (đừng viết thừa — 14 form đã convert đều không có). Field ngày trong DÒNG bảng
    (`expiry_date`/`manufacturing_date` per-row): **phải tự convert** trong `getPayload()`/`buildFormData()`.
    Chỉ giữ thủ công phần thật sự cần: fallback `now()` cho create-mode nếu bản cũ có (`this.today` kế
    thừa từ `BaseApp`).
34. **`[disabled]` trên element có `formControlName` là NO-OP** — đã đọc `node_modules/@angular/forms/
    fesm2022/forms.mjs`: directive nhận input `disabled` rồi **chỉ `console.warn`**, KHÔNG disable control.
    ⇒ Khi audit payload lúc convert, **KHÔNG được** kết luận "bản cũ không gửi field X vì template có
    `[disabled]`" — field đó VẪN nằm trong `rfModel.value` và VẪN được gửi. Phải đọc `getPayload()` thật.
    Bản mới tái hiện phần khoá UI bằng `readonly()`/`disabled(field, {when})` trong `buildSchema()`
    (KHÔNG attribute thô — gotcha #11) và giữ NGUYÊN tập key payload.
35. **"BE gán cứng field X" phải verify theo TỪNG HANDLER, không theo resource.** Ca thật
    (`customer_prescription`): `UpdateCustomerPrescription` gán cứng `body.AppointmentId = item.AppointmentId`
    (client gửi gì cũng vô hiệu), NHƯNG `CreateCustomerPrescription` **ĐỌC `appointment_id` từ payload**
    để gắn đơn thuốc vào lần khám. Whitelist `getPayload()` suy ra từ mỗi handler Update sẽ **âm thầm mất
    liên kết ở đường tạo mới** — `tsc`, `ng build`, và cả test tay đường Sửa đều KHÔNG lộ ra.
36. **BE ghi cột string qua `fmt.Sprintf("%v", v)` biến JSON `null` thành chuỗi literal `"<nil>"` trong
    DB** (gặp ở mọi `SaveDetails` của module med). Đây là lý do ĐỘC LẬP với gotcha #9 buộc field text phải
    seed `''`: gotcha #9 nói về phía FE (parse số, mất số 0 đầu), gotcha này nói về phía DB (rác `"<nil>"`
    lưu vĩnh viễn). Field text gửi lên phải quy đổi `null → ''` ngay trong `getPayload()`, không chỉ ở
    hàm seed model.
37. **`BaseSignalForm<any>` phá truy cập property trên `rootField`** — mở rộng của gotcha #4. Form không
    map model thật (vd form import Excel) buộc dùng `<any>` vì `modelService: BaseApiService<T>` chỉ nhận
    service thật khi `T = any`; khi đó `FieldTree<any>` dính quirk "conditional type phân phối trên `any`"
    ngay ở **truy cập property** (`rootField.branch_id` → `TS2339`), không chỉ ở tham số hàm. Fix: khai
    interface model cục bộ + getter
    `get f(): FieldTree<LocalModel> { return this.rootField as unknown as FieldTree<LocalModel>; }`, mọi
    chỗ dùng `f.x!` thay `rootField.x!` (cả template lẫn component).

38. **[HIGH] `BaseSignalForm` KHÔNG tự chạy `setupFormValueTracking()` như `BaseForm`** — form Signal
    Forms phải **tự gọi `trackScopeFields('branch_id')`** trong constructor (bản cũ nhận cả `'company_id'` — đã bỏ, D15). Thiếu thì
    `currentBranchId()` luôn `undefined` → mọi control lọc theo branch (chi nhánh, phòng ban,
    loại chăm sóc…) lọc SAI hoặc rỗng. Không lỗi compile, không lỗi runtime, không cảnh báo — chỉ lộ khi
    mở dropdown trên browser thật. Phát hiện ở epic `care-signal-forms` story-01 (`CustomerCareListForm`),
    áp lại story-02. Kiểm mục này ở MỌI form convert có field `branch_id` (`company_id` đã bỏ, D15).

39. **[CRITICAL] BE dùng `Updates(map[string]interface{})` thì key payload = TÊN CỘT — key lạ làm VỠ SQL,
    không phải bị bỏ qua.** Nặng hơn hẳn nhóm handler `Save()` struct (ở đó `encoding/json` chỉ lờ key
    thừa đi). Mà `dialogConfig.data.model` thường là row **đã qua `addLabel()`** của list — đầy key hiển
    thị (`customerDisplayName`, `branchLabel`, `staff`, `*Info`, `*Formatted`) và cả object quan hệ
    (`customer`) → `extractValue(rootField)` mặc định đẩy hết lên BE → **`Error 1054 Unknown column`,
    hỏng hẳn nút Lưu** (không phải lỗi âm thầm, nhưng lỗi 100% ở mọi lần Sửa). Bắt buộc `getPayload()`
    whitelist tường minh, và đối chiếu từng key với **cột thật trong migration**, không chỉ với model FE.
    Lưu ý BE có thể xoá hộ vài key FE-only (`pkg/helpers/helpers.go` — `feOnlyFields`, hiện chỉ
    `customer_data`) và nhánh CREATE có thể `delete()` nhiều key hơn nhánh UPDATE — **đọc CẢ HAI nhánh**,
    đừng suy ra từ một nhánh (ca thật: `customer_care_complaints` CREATE có `delete(body,"customer")`,
    UPDATE thì không). Epic `care-signal-forms`, cả 5 form.

40. **Danh sách cột SELECT của list-API ≠ cột thật của bảng — field vắng trong list sẽ bị ghi đè TRẮNG
    sau khi convert.** Cơ chế: cột tồn tại thật trong DB nhưng list không SELECT → `data.model` thiếu key
    → Reactive Forms cũ để `undefined` → `JSON.stringify` bỏ key → DB giữ nguyên (vô tình đúng). Signal
    Forms cấm `undefined` (gotcha #1) và ép seed `''`/`null` (gotcha #9) → key XUẤT HIỆN trong payload →
    `Updates` **ghi đè trắng cột đó mỗi lần Sửa**. Ca thật: `customer_care.name` ("Tiêu đề") — CẢ HAI
    endpoint list (`ListCustomerCare`, `ListCareOfCustomer`) đều không SELECT, chỉ `GetCustomerCare`
    (`SELECT *`) mới có. Fix: form gọi `GET(id)` khi Sửa (gotcha #15; chi phí 1 request được
    `api-dedupe.interceptor.ts` hấp thụ — gotcha #23). → Khi convert: **liệt kê cột SELECT của đúng
    endpoint list mà form đó dùng** rồi so với tập control; đừng giả định 2 endpoint list của cùng
    resource giống nhau. Epic `care-signal-forms` story-01/02.

41. **[CRITICAL] Field text rỗng: `''` KHÔNG phân biệt được "chưa chạm" với "user cố ý xoá" — dùng
    `dirty()` để khôi phục.** Mở rộng trực tiếp của #9 × #19, gặp ở cả 8 form `crm/payment`.
    Reactive Forms phân biệt tự nhiên: control chưa chạm (model `null`) → `.value` = `null` → BE giữ DB;
    user xoá trắng ô → `''` → BE ghi đè rỗng. Signal Forms **buộc seed `''`** (gotcha #9) nên **cả hai
    tình huống đều ra `''`** → mất khả năng phân biệt. Quy `'' → null` vô điều kiện thì an toàn dữ liệu
    nhưng **user không xoá được ghi chú/mã ngân hàng nữa** (im lặng, không ai báo).
    → Dùng `dirty()` của chính field trong `getPayload()`:
    ```ts
    for (const key of TEXT_INPUT_FIELDS) {
      if (payload[key] === '' && !this.rootField[key]?.().dirty()) payload[key] = null;
    }
    ```
    `''` + CHƯA dirty → `null` (chưa chạm → BE giữ DB) · `''` + ĐÃ dirty → `''` (cố ý xoá → ghi đè).
    Chỉ cần khi BE merge-by-presence trên field **con trỏ** (`*string`): ở đó `null` ≡ vắng key.
    (Epic `crm-signal-forms` đợt 2, wave A — `form-deposit.ts` là bản mẫu.)

42. **[HIGH] `.value` hay `.getRawValue()` — phải ĐỌC lời gọi payload thật, đừng suy từ việc có
    `disable()` hay không.** Hệ quả ngược nhau hoàn toàn:
    `FormGroup.value` **loại** key control `disabled`; `getRawValue()` **giữ**. `extractValue()` của
    Signal Forms luôn **giữ**. ⇒ Form dùng `.value` + có `disable()` → phải tự loại key (nếu không payload
    mọc key → ghi đè DB). Form dùng `getRawValue()` → **giữ nguyên**, loại key đi mới là tạo bug.
    Ca thật trong 1 epic: `switch-form`/`warranty-card-form` dùng `.value` (phải loại key), còn
    `form-move-deposit`/`form-installment` dùng `getRawValue()` (giữ) — mà `form-installment` field bị
    khoá chính là **ô SỐ TIỀN**, loại nhầm là mất tiền trong DB. Cùng họ với #34.

43. **[HIGH] `readonly()` CÓ tồn tại — đừng thay bằng `disabled()`.** Tài liệu cũ (checklist epic
    `config-signal-forms`) ghi "Signal Forms không có `readonly()`, chỉ có `disabled()`" — **đã lỗi thời**.
    `readonly` export sẵn từ `@angular/forms/signals`. Khác biệt có thật:
    | | `disabled(field)` | `readonly(field)` |
    |---|---|---|
    | Hiển thị | ô bị **xám** | như `readonly` HTML (không xám) |
    | Field trong model/payload | vẫn có | vẫn có |
    | Tương ứng Reactive Forms | `control.disable()` | attribute `readonly` |
    ⇒ Field bản cũ khoá bằng **attribute `readonly`** phải dùng `readonly()`; dùng `disabled()` là **đổi
    giao diện**. Field bản cũ gọi `control.disable()` mới dùng `disabled()` — và nhớ #42 cho payload.

44. **6 member của `BaseForm` KHÔNG tồn tại ở `BaseSignalForm`** (nó kế thừa `BaseApp`, không phải
    `BaseForm`). Đa số **chỉ lộ ở `ng build`** (`TS2339` trong template), `tsc --noEmit` im lặng:
    | Member | Cách thay |
    |---|---|
    | `handleSelectedDate()` / `formatDate()` | hàm parse THUẦN module-level (#22) |
    | `canEditBranch` (signal) | khai lại `protected readonly canEditBranch = signal(true)` nếu form không bao giờ set `false` |
    | `setupFormValueTracking()` | tự gọi `trackScopeFields('branch_id')` (#38; vế `company_id`/`currentCompanyId` đã bỏ, D15) |
    | `f` (getter `rfModel.controls`) | `f['x'].errors` → `isFieldInvalid(rootField.x!)`; `f['x'].value` → `rootField.x!().value()` |
    | `customValidationService` | tự `inject(UtilsService)` rồi `isEmail()` + `emailError()` |
    | `normalizeDateField()` | viết lại rút gọn cho đúng field cần |
    Kèm 2 xung đột TÊN với base (bắt được bởi `tsc`, `TS2415`/`TS4114`): `private injector` và
    `userCacheService` — lớp con phải đặt tên khác (`formInjector`, `paymentInjector`…).

45. **Cascade `valueChanges` thường nằm TRONG `buildForm()` — xoá `buildForm` là xoá luôn cascade.**
    Khi convert, `buildForm()` bị thay bằng `buildSchema()`, nhưng `buildForm()` ở nhiều form còn chứa
    `valueChanges`, lời gọi API nạp dropdown, và giá trị khởi tạo. **Kiểm template xem có handler nào gọi
    trực tiếp không** (`(onChange)`/`(valueChange)`): có → cascade đó đã được template lo; **không** →
    phải đăng ký lại bằng `effectOnChange` trong constructor, nếu bỏ sót thì lỗi ÂM THẦM (vd đổi phương
    thức thanh toán mà không reset loại thẻ). Cascade có `debounceTime` thì giữ nguyên debounce bằng
    `toObservable(computed(() => field().value()))` + `debounceTime(n)` (đừng đổi thành `effect` trần —
    B.19: gõ dở số sẽ tính lại theo từng keystroke).

46. **Signal Forms KHÔNG có `markAsPristine()`/`markAsUntouched()`** — chỉ có `markAsTouched()`
    (`FieldState` có getter `dirty`/`pristine`/`touched`/`untouched` nhưng không có hàm đánh dấu ngược).
    Form nào bản cũ gọi 2 hàm này sau một thao tác "xoá hết/reset" sẽ **hiện `AppConfirmDialog` khi đóng**
    dù người dùng thấy như chưa thay đổi gì. Cần đúng hành vi cũ → dùng `createPendingTracker()`
    (`share/utils/signal-form-pending.ts`, xem `frontend/skill/skills/building-angular-signal-forms/SKILL.md`) thay vì dựa `dirty()`.

47. **[HIGH] View-model riêng cho form: field phải để `optional`/union đủ rộng để API model assignable
    sang nó — `BaseApiService<T>` BẤT BIẾN theo `T`.** Nó chứa `WritableSignal<T | null>` nên
    `modelService = inject(XxxService)` chỉ hợp lệ khi `XxxModel` assignable sang view-model; lệch 1
    field là `TS2416` ngay tại dòng `modelService` (bắt được bởi `tsc`, thông báo dài dòng trỏ vào
    `Types of property 'model' are incompatible`). Ca thật (field đã bỏ, D15 — cơ chế vẫn đúng): `company_id: number | null` vs `number | undefined` của API model → phải khai `company_id?: number | null`.
    ```ts
    interface DelegateFormModel {          // Date/boolean cho control, number/0-1 cho API
      id?: number; to_user_id: number | null; doc_type?: string | null;
      from_at: number | Date | null; to_at: number | Date | null; status?: number | boolean;
    }
    ```
    Union rộng là cách giữ kiểu thật; `as unknown as BaseApiService<T>` chỉ dùng khi không thể nới.
    Đọc `model()` dạng API model trong `ngOnInit()` cần `as unknown as Partial<XxxModel>` (2 kiểu
    không đủ overlap → `TS2352`). Khi nào dùng view-model riêng thay vì API model: field lệch kiểu
    giữa control và API (`p-datepicker` ↔ unix, `p-toggleswitch` ↔ 0/1) mà nới thẳng API model sẽ vỡ
    AOT ở list/detail dùng chung (#24).

48. **Form dùng `BaseSignalForm` nhưng KHÔNG CRUD model nào (dialog lọc/bulk, dialog thao tác) phải
    override `ngOnInit()` seed model riêng** — đường mặc định seed bằng `modelService.getInitialObject()`,
    tức shape của resource khác hẳn, mọi `rootField.x!()` vỡ theo #1. `buildSchema()` rỗng là hợp lệ
    (form không bind `[formField]` field nào — vd dialog kiểm kê chạy `[(ngModel)]` trên object thuần).
    Vẫn phải khai `modelService` (abstract) — trỏ service nào cũng được miễn qua được #47.
    Mẫu: `FormExportReport`, `FormBulkInsert` (unit), `ServiceTypeFormBulkInsert`, `CountStockTake`.

49. **Validator bê từ Reactive Forms sang phải đối chiếu BE + DDL trước khi giữ.** Mặc định là GIỮ
    NGUYÊN (#10 — `required()` không coi `[]` là rỗng, phải `validate()`+`requiredError()`), nhưng
    convert là lúc duy nhất soi lại: có ràng buộc vốn **vô căn cứ**, chặn nút Lưu im lặng không báo gì.
    Ca thật (`FormBulkInsert` unit): 2 field `Validators.required` là **bộ lọc client-side thuần**, không
    nằm trong payload — `BulkInsertUnit` chỉ đọc `name`/`description`/`categoryId` từng dòng đã tick,
    bảng `unit` không có cột `departmentTypes`, `categoryId` là `json DEFAULT NULL`. Đối chứng cùng
    epic: `service_type.company_id` là `int NOT NULL` + FK và BE chặn tường minh → GIỮ `required` (lịch sử — cột đã bỏ, D15).
    → Quy tắc: field KHÔNG vào payload thì không có cơ sở `required`; field có vào payload thì tra cột
    thật trong migration + nhánh validate của handler. Bỏ validator là đổi hành vi → tách commit riêng,
    đừng gộp vào commit convert.


50. **Gỡ `ReactiveFormsModule` mà giữ `(ngSubmit)` trong template = nút Lưu chết + MẤT DỮ LIỆU.**
    `(ngSubmit)` là **output của directive `NgForm`** (đến từ `FormsModule`/`ReactiveFormsModule`).
    Signal Forms KHÔNG có directive nào gắn lên `<form>`, nên sau khi convert `(ngSubmit)` trở thành
    listener cho một DOM event tên "ngSubmit" **không bao giờ bắn** — và không ai gọi `preventDefault()`
    nữa. Kết quả: bấm nút `type="submit"` → trình duyệt **submit native (GET)**, điều hướng sang
    `?ng.form0.<field>=...` (lộ nguyên model lên URL), form remount trắng, dữ liệu vừa nhập mất sạch,
    **không có request nào tới BE**. `tsc` + `ng build` đều XANH; chỉ lộ khi bấm Lưu thật.
    → Sửa: `<form (submit)="$event.preventDefault(); onSave()">`. Giữ luôn hành vi Enter-để-lưu như bản
    Reactive Forms cũ.
    → Grep bắt buộc sau MỌI đợt convert (chạy từ `frontend/`):
    ```bash
    for f in $(grep -rln "ngSubmit" src/app --include="*.html"); do ts="${f%.html}.ts"; [ -f "$ts" ] || continue;
      grep -q "@angular/forms/signals" "$ts" && ! grep -qE "ReactiveFormsModule|FormsModule" "$ts" && echo "$f"; done
    ```
    Phải = 0. File có `type="submit"` là vỡ chắc chắn; file dùng `type="button"`+`(click)` thì nút Lưu
    vẫn chạy nhưng gõ Enter trong ô text vẫn kích submit native → cũng phải sửa.
    Gặp thật 2026-08-12: 8 form EMR (epic emr-signal-forms + emr-signal-forms-examination), phát hiện khi
    smoke test tay chứ QA/build không bắt được.
51. **[HIGH] Form có mảng dòng: side-state per-row PHẢI key theo `_uid`, KHÔNG theo `index`.**
    Mở rộng của #18 (`[rowTrackBy]` theo `_uid`): cùng một khoá ổn định đó phải dùng cho **mọi Map/mảng
    phụ trợ bám theo dòng** (state UI, cờ hiển thị, tooltip, cấu hình trả góp, chữ ký chống-tính-đè…),
    chứ không chỉ cho `trackBy`. Key theo `index` bắt `deleteXxx()` phải **dịch tay toàn bộ map** sau mỗi
    lần xoá (`k < key ? k : k-1`) — sót 1 map là state gán nhầm sang dịch vụ/dòng khác, im lặng, build xanh.
    → Key theo `_uid` thì xoá dòng chỉ còn `map.delete(uid)`; các dòng còn lại giữ nguyên khoá.
    → Quy ước đặt tên tham số cho rõ ranh giới: `index` = **vị trí** (chỉ dùng cho thao tác thực sự theo
    vị trí: patch dòng thứ i, xoá dòng thứ i), `uid` = **danh tính** dòng (mọi side-state). Hàm nhận
    `index` từ component con (autocomplete emit `index`) thì quy đổi sang `_uid` ngay dòng đầu.
    → Template: `@let uid = $any(rowData)?._uid;` ngay đầu row body rồi dùng `uid` cho mọi call site
    (nhớ `?.` — `@let` truy cập thẳng thuộc tính, rowData null là throw vỡ cả bảng).
    → **Callback BẤT ĐỒNG BỘ (dialog `onClose`, API response) TUYỆT ĐỐI không dùng lại `index` chụp lúc
    mở** — user xoá/thêm dòng khác trong lúc chờ là patch nhầm dòng. Tra lại: `findIndex(r => r._uid === uid)`;
    không còn thì báo cho user, đừng `return` câm (BE có thể đã commit rồi).
    → `getPayload()` loại `_uid` khỏi payload (FE-only), nên chỗ nào cần zip payload ngược về state theo
    `_uid` thì **chụp mảng model ngay cạnh `getPayload()`**, không đọc lại `model()` ở tầng sâu bên trong
    `.map()` — chèn bất kỳ lệnh ghi `model` nào vào giữa là lệch map, không có gì bắt được.
    Ca thật (`OrderPlanForm`, epic `order-plan-form-rekey-uid` 2026-08-12): 7 side-state + ~30 dòng
    re-index thủ công. Hậu quả nếu lệch: BE `order_treatment_plan.go:1149` **xoá thẳng**
    `order_installment` của dòng nào payload thiếu `installment_config` → mất kỳ trả góp của dịch vụ A,
    tạo mới cho B, không báo lỗi.

52. **[CRITICAL] `model` của `BaseSignalForm` mặc định `{}` (TRUTHY) — `override ngOnInit()` phải phân
    nhánh create/edit bằng `isEditMode()`, CẤM `if (this.model())`.** Hai base class ngược nhau ở
    constructor, cùng một dòng code cho kết quả trái dấu:
    ```ts
    // base-form.ts (Reactive) — set VÔ ĐIỀU KIỆN
    this.model.set(structuredClone(this.dialogConfig?.data?.model));  // không có → undefined → if() FALSE ✅
    // base-signal-form.ts — CÓ guard (cố ý, chống #1: model {} làm rootField.x = undefined, template vỡ)
    if (modelParam && Object.keys(modelParam).length > 0) this.model.set(modelParam);  // không có → giữ {} → if() TRUE ❌
    ```
    ⇒ Form bê nguyên `if (this.model())` từ bản cũ sang sẽ chạy **nhánh Sửa lúc Tạo mới**: khối seed
    `initialValue` không bao giờ chạy. Field required nào **không render ở template** (FK ngầm:
    `customer_id`, `payment_for`, … — ca gốc còn `company_id`, đã bỏ D15) sẽ rỗng → `rootField().invalid()` luôn true →
    `onSubmit()` `return` câm: **không request, không toast, không ô nào đỏ**. `tsc` + `ng build` + QA
    đọc code đều XANH; chỉ lộ khi bấm Lưu thật. Ca thật `FormPayment` (2026-08-13, epic
    `crm-signal-forms`) — 1/211 form lọt lưới, 6 form inventory đã có comment `[CRITICAL]` cảnh báo đúng
    bẫy này từ trước. Grep chốt sau MỌI đợt convert (chạy từ `frontend/`, phải = 0):
    ```bash
    grep -rn "if (this\.model())" src/app --include='*.ts' \
      | while IFS=: read -r f _; do grep -q "extends BaseSignalForm" "$f" && echo "$f"; done
    ```
    → Hệ quả kèm theo (kiểm luôn khi dính): field required **không có ô nhập trên template** thì user
    không bao giờ thấy lỗi. Mọi field `required()` CÓ render phải bind
    `[class.ng-invalid]="isFieldInvalid(rootField.x!)"` (convention repo, xem `card-type-form.html`) —
    thiếu binding = nút Lưu chết câm y hệt, kể cả khi validator hoàn toàn đúng. Cũng ở `FormPayment`:
    `branch_id`/`method`/`card_type_id` thiếu binding này (nợ sẵn từ bản Reactive, convert không tạo ra
    nhưng cũng không xoá).

53. **[HIGH] `p-editor` (Quill) cho phép dán ảnh → TỰ nhúng base64 `data:image/...` vào giá trị HTML,
    không hỏi trước.** Cột DB chỉ TEXT/MEDIUMTEXT — base64 dễ vượt giới hạn hoặc phình payload vô ích.
    KHÔNG fix bằng cách widen cột — chặn tại nguồn: mọi field bind `p-editor` PHẢI khai trong
    `richTextImageFields()` (dùng `onSubmit()` mặc định) hoặc gọi
    `this.submitWithRichTextImages(['field'], () => {...})` ngay đầu `onSubmit()` tự override, trước
    khi build payload — upload ảnh lên S3 qua `EditorImageService.replaceBase64ImagesInHtml()`
    (endpoint có sẵn `POST crm/tinymce/upload-image`), ghi URL ngược vào field rồi mới submit.

54. **Ghi giá trị bằng code (`field().value.set(v)`, `model.update(...)`) KHÔNG làm field `dirty`.**
    Chỉ thao tác người dùng trên control mới bật cờ. Form có chèn nội dung bằng code (chèn biến tại con
    trỏ, ô bind `[ngModel]`+`model.update`) mà dựa `dirty()` để hỏi "huỷ thay đổi?" ⇒ lọt. Kiểm "có thay
    đổi chưa" bằng so GIÁ TRỊ payload hiện tại với bản seed (khoá sắp xếp, `''` ≡ `null`) — mẫu
    `examTemplateDataChanged` (`config-emr/exam-template/form/exam-template-content-form.logic.ts`).

55. **Nhánh edit `BaseSignalForm.ngOnInit` seed model RỖNG trước, GET về mới `model.set(loaded)`** ⇒
    người dùng gõ trong lúc chờ (mạng chậm) bị dữ liệu thật ghi đè mất; bấm Lưu lúc đó thì PUT form rỗng.
    Form mở theo id có nội dung dài: `loaded = computed(() => !this.id() || !!this.model().id)` — chưa
    `loaded` thì template hiện skeleton thay cho form, `onSubmit()` return sớm.
    ⚠ GET-by-id trong base `ngOnInit` KHÔNG có nhánh error ⇒ dùng skeleton `loaded` mà GET lỗi (404 bản ghi
    đã bị xoá, 5xx) là kẹt skeleton mãi. Form tự GET (vd `service.getForEdit(id)` gắn `SKIP_GLOBAL_404`) và
    đóng dialog + báo lỗi khi lỗi — không sửa base dùng chung.
