# Frontend forms UX

## Mục lục

  - B.13 Form validation — hiển thị lỗi ngay khi render (HIGH)
  - B.x Reactive form — KHÔNG dùng `[disabled]` trên element có `formControlName`
  - B.15 Các component PrimeNG KHÔNG có trong `FormShareImports`
  - B.16 BaseReport — `rfFilter` phải là single source of truth
  - B.17 UX mượt mà — perceived performance (HIGH)
  - B.19b Form resilience bổ sung (Vercel Web Interface Guidelines — lọc theo ERP)
  - B.19c Form UX bổ sung (nguồn: `nextlevelbuilder/ui-ux-pro-max-skill`, MIT — lọc theo ERP desktop)

---


Tách từ `frontend/skill/skills/frontend-hard-rules/SKILL.md` (giữ file chính ≤ 350 dòng). Rule B.13, B.15–B.17 + B.x.

### B.13 Form validation — hiển thị lỗi ngay khi render (HIGH)

**KHÔNG thêm gate `touched || dirty || submitted()`** vào `isFieldInvalid()` / `isGroupFieldInvalid()`.

**Phạm vi rule này = lỗi cấp FIELD.** Chỉ báo tổng hợp cấp TAB/nhóm (chấm đỏ trên header tab, badge "tab nào đang có lỗi") là affordance khác — **được phép và nên** gate `touched || submitted()`, nếu không form vừa mở đã đỏ khi user chưa gõ gì. Đừng "dọn cho đồng nhất" khi convert sang Signal Forms (gặp thật ở `PatientsForm.tabErrors`, epic `emr-signal-forms`).

```typescript
// ✅ ALWAYS
protected isFieldInvalid(field: string): boolean {
  const c = this.rfModel?.get(field);
  return !!c && c.invalid;
}

// ❌ NEVER
protected isFieldInvalid(field: string): boolean {
  const c = this.rfModel?.get(field);
  return !!c && c.invalid && (c.dirty || c.touched || this.submitted());
}
```

```html
<!-- ✅ -->
<label [class.text-red-500]="isFieldInvalid('name')">Họ tên *</label>
<input [class.ng-invalid]="isFieldInvalid('name')" formControlName="name" />

<!-- ❌ NEVER -->
<label [class.text-red-500]="f['name'].touched && f['name'].invalid">Họ tên *</label>
```

**Áp dụng CẢ cho form không dùng reactive `FormGroup`** (component tự quản field bằng signal riêng, vd dialog nhỏ dùng `[ngModel]`/`(ngModelChange)` thay vì `formControlName`) — không có `isFieldInvalid()` có sẵn từ `BaseForm` thì tự viết `computed()` tương đương, KHÔNG chỉ thêm dấu `*` rồi bỏ qua bước tô đỏ. Dấu `*` chỉ là gợi ý phụ — tín hiệu chính để user biết "phải điền" là màu đỏ hiện NGAY khi mở form, không cần chờ blur/touch:

```typescript
// Form signal-based (không có rfModel) — tự định nghĩa invalid computed, cùng nguyên tắc
// "không gate touched" như B.13
protected readonly policyInvalid = computed(() => !this.policyId());
protected readonly fromDateInvalid = computed(() => !this.fromDate());
```
```html
<label [class.text-red-500]="policyInvalid()">Loại phép *</label>
```

### B.x Reactive form — KHÔNG dùng `[disabled]` trên element có `formControlName`

`[disabled]` binding trên element có `formControlName` gây "changed after checked" warning và bị Angular ignore trong một số trường hợp. Dùng form API:

```typescript
// Tại khởi tạo control
fb.group({ service_id: [{value: item?.service_id, disabled: this.isLocked}] })

// Sau khi init (vd sau khi load data)
(formArray.at(i) as FormGroup).get('service_id')?.disable();

// Reactive theo signal
effect(() => { this.isLocked() ? ctrl.disable() : ctrl.enable() });
```

Khi control bị disable qua form API, Angular tự gọi `setDisabledState()` trên ControlValueAccessor của custom component — không cần `[disabled]` riêng.

### B.15 Các component PrimeNG KHÔNG có trong `FormShareImports`

`FormShareImports` (dùng cho dialog/form) THIẾU một số component so với `ListShareImports`:

| Component | `ListShareImports` | `FormShareImports` |
|-----------|--------------------|--------------------|
| `IconField` (`p-iconfield`) | ✅ | ❌ |
| `InputIcon` (`p-inputicon`) | ✅ | ❌ |
| `ConfirmDialogModule` (`p-confirmDialog`) | ✅ | ❌ |
| `InputNumber` (`p-inputNumber`) | ❌ | ✅ |

**Rule:** Create page (full page, không phải dialog) dùng `ListShareImports`. Dialog/form cần `p-iconfield`/`p-inputicon`/`p-confirmDialog` → import riêng hoặc đổi sang `ListShareImports`.

```typescript
// Create page dùng ListShareImports — có đủ cả 3
@Component({ imports: [...ListShareImports] })

// Dialog/form cần thêm thủ công:
import {IconField} from 'primeng/iconfield';
import {InputIcon} from 'primeng/inputicon';
```

### B.16 BaseReport — `rfFilter` phải là single source of truth

KHÔNG tạo class property song song (`asOfDate: Date | null`, `warehouseId: number | null`). `fetchData()` đọc từ `this.rfFilter.value`, không từ class property.

```typescript
// ✅ CORRECT
rfFilter = this.fb.group({
  warehouse_id: [null as number | null],
  as_of_date: [null as Date | null],
});

ngOnInit() {
  this.rfFilter.valueChanges.pipe(debounceTime(500), takeUntilDestroyed(this.destroyRef))
    .subscribe(() => this.loadReport());
  this.loadReport();
}

reset() {
  this.rfFilter.reset({warehouse_id: null, as_of_date: null}, {emitEvent: false});
  this.loadReport(); // emitEvent: false → gọi trực tiếp, tránh double-load
}

fetchData() {
  const v = this.rfFilter.value;
  return this.svc.valuation(v.as_of_date, v.warehouse_id ?? undefined);
}

// ❌ AVOID
asOfDate: Date | null = null; // duplicate state bên ngoài rfFilter
reset() { this.asOfDate = null; /* rfFilter vẫn giữ giá trị cũ */ }
```

### B.17 UX mượt mà — perceived performance (HIGH)

**Nguyên tắc chung:** Mọi feedback trạng thái (loading, toast, dialog open/close, disable/enable) phải transition mượt, không snap tức thời — và không hiện feedback nếu thao tác đã xong quá nhanh để mắt kịp nhận biết.

**1. Loading indicator — debounce show + min-visible-duration**

```typescript
// ❌ NEVER — bật/tắt tức thời theo request, API nhanh (<150ms) gây chớp nháy
isLoading = computed(() => this.totalCount() > 0);

// ✅ ALWAYS — chỉ hiện nếu kéo dài > 150ms, đã hiện thì giữ tối thiểu 250ms
// Implementation chuẩn: share/services/global-loading.service.ts
```
Show delay ~150ms, min-visible ~250ms — áp dụng cho MỌI spinner/overlay mới, không riêng `GlobalLoadingService`. DOM show/hide phải có `transition: opacity`, KHÔNG `@if` toggle cứng không transition.

**2. Feedback gắn liền hành động, không tách rời global overlay**

```html
<!-- ❌ NEVER -->
<button (click)="onSave()">Lưu</button>

<!-- ✅ ALWAYS — spinner ngay trên nút, disable trong lúc submit -->
<button [disabled]="isSubmitting()" (click)="onSave()">
  <i class="pi" [class.pi-spin]="isSubmitting()" [class.pi-spinner]="isSubmitting()"></i>
  Lưu
</button>
```
Global loading overlay chỉ là tín hiệu phụ (navigation, sync ngầm). Hành động user chủ động bấm (save/submit/delete) LUÔN có feedback cục bộ ngay tại điểm tương tác.

**3. Tránh layout shift (CLS)** — list/table/card load lần đầu dùng `p-skeleton` cùng kích thước, KHÔNG spinner giữa màn hình đẩy layout.

**4. Optimistic UI** cho thao tác nhỏ ít rủi ro rollback (toggle, favorite, đổi status đơn giản) — cập nhật UI ngay, rollback + toast lỗi nếu request fail.

**5. Không chồng loading indicator** — nếu button đã có spinner riêng (rule #2) thì KHÔNG để `GlobalLoadingService` cũng overlay cho cùng request.

**6. Dialog/drawer open-close phải animate** — KHÔNG override `[showTransitionOptions]="'0ms'"`. Đóng dialog sau submit → đợi 1 frame (`requestAnimationFrame`) trước khi toast hiện, tránh 2 animation chồng giật cùng lúc.

Custom animation cho `p-dialog` (PrimeNG v22+, CSS-class-based motion) — sửa ở `frontend/src/assets/overrides/custom.scss` (source, KHÔNG sửa `public/assets/overrides/`), override global class `.p-dialog-enter-active`/`.p-dialog-leave-active`:

```scss
// !important BẮT BUỘC — PrimeNG tự inject CSS bằng JS lúc component khởi tạo,
// thường SAU khi styles.scss đã load → cùng specificity thì rule PrimeNG thắng nếu thiếu !important.
.p-dialog-enter-active {
  animation: cluvix-dialog-in 250ms ease-out !important;
}
.p-dialog-leave-active {
  animation: cluvix-dialog-out 300ms ease-in !important;
}
@keyframes cluvix-dialog-in { from { opacity: 0; transform: translateY(-12px); } }
@keyframes cluvix-dialog-out { to { opacity: 0; transform: translateY(200%) rotate(-90deg); } }
```

**KHÔNG animate `filter: blur`/`scale` trên container có text user cần đọc (enter animation)** — text bị rasterize lên GPU compositing layer, mất subpixel antialiasing → user thấy chữ mờ mỗi lần mở dialog. Enter chỉ fade + `translateY` nhỏ. Leave animation dùng hiệu ứng mạnh (rotate/translateY lớn) ĐƯỢC — user không đọc text khi dialog đang biến mất, GPU layer giải phóng ngay khi remove khỏi DOM.

Chỉ sửa `.scss` KHÔNG thấy hiệu ứng đổi dù build/tsc pass → gần như luôn là thiếu `!important` (không phải bug code) — verify bằng cách thêm `!important` trước khi điều tra sâu hơn.

**7. Debounce input tương tác liên tục** — search/filter/autocomplete `debounceTime(300)` (xem checklist ở `frontend/skill/skills/frontend-hard-rules/SKILL.md`).

### B.19b Form resilience bổ sung (Vercel Web Interface Guidelines — lọc theo ERP)

- Input cần `autocomplete` + `type` đúng nghĩa (`email`, `tel`, `number`) — autofill trình duyệt và bàn phím mobile hiện đúng loại.
- Checkbox/radio: label và control chung 1 hit target (wrap trong `<label>`), không để label tách rời input gây dead zone click.
- Submit lỗi validation: focus vào field lỗi đầu tiên, không chỉ hiện toast chung chung — kết hợp `isFieldInvalid()` (B.13) để scroll/focus control đầu tiên có `invalid`.
- Form dài có khả năng mất dữ liệu khi rời trang (bệnh án, đơn thuốc, hồ sơ nhiều tab) → cảnh báo unsaved changes qua `CanDeactivate` guard hoặc `beforeunload`, không để user mất dữ liệu vì bấm back/đóng tab nhầm.
- Thao tác xoá/huỷ luôn qua `p-confirmDialog` — kể cả xoá 1 dòng trong bảng con (FormArray), KHÔNG xoá ngay khi click.

### B.19c Form UX bổ sung (nguồn: `nextlevelbuilder/ui-ux-pro-max-skill`, MIT — lọc theo ERP desktop)

- **`read-only-distinction` — read-only KHÁC disabled**: disabled = "không dùng được lúc này" (mờ, ngoài tab order); read-only = "giá trị có thật, xem/copy được, không sửa được". Đừng dùng disabled cho giá trị chỉ-đọc (user tưởng bị khoá quyền), và đừng dùng `<input readonly>` cho giá trị để copy — chi tiết + mẫu đúng ở `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.19b.
- **`sheet-dismiss-confirm` — dialog đóng khi form còn dirty**: `CanDeactivate`/`beforeunload` chỉ chặn ở tầng ROUTE, KHÔNG bắt được dialog đóng bằng nút X / click overlay / phím Esc (`DynamicDialogRef`). Dialog form dài (bệnh án, đơn hàng nhiều dòng) phải tự chặn 3 đường này khi `form.dirty`, không thì user click ra ngoài là mất sạch mà không có cảnh báo nào.
- **`undo-support` cho thao tác hàng loạt**: xoá/đổi trạng thái nhiều bản ghi cùng lúc nên có đường lùi (toast "Hoàn tác" trong 5-10s hoặc soft-delete khôi phục được). Confirm dialog chỉ chặn được click nhầm, không cứu được chọn nhầm 50 dòng.
- **`input-helper-text`**: field có quy tắc không hiển nhiên (định dạng mã, ràng buộc nghiệp vụ, đơn vị tính) cần helper text THƯỜNG TRỰC dưới field — không nhét vào `placeholder` (biến mất ngay khi gõ, đúng lúc user cần nhất).
- **`password-toggle`**: field mật khẩu / secret key phải có nút mắt bật/tắt hiển thị để user tự soát lại chuỗi vừa dán. Trong repo dùng pattern của trang login (`modules/auth/login/login.ts` — input thường + nút icon `position:absolute` đè bên phải, `pr-10` chừa chỗ), KHÔNG dùng `p-password` `[toggleMask]` (xem gap đã ghi ở `frontend/skill/skills/using-primeng-components/SKILL.md`). Mẫu đã áp: `config/config-zalo/zalo-oa-config/dialog/zalo-oa-connect-dialog.html`.
