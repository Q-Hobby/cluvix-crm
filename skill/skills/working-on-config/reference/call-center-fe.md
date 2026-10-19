# Call Center FE

## Mục lục

- FE — `call-center-provider.ts`
- FE — `call-center-provider-form.ts`
  - Omicall config_system structure
  - Hotlines FormArray
- FE — `call-center-extensions.ts`
  - Omicall extensions dropdown
  - extControls getter — PHẢI trả new reference
  - Luôn có error handler cho API calls
  - onSubmit gửi config_system masked → an toàn
  - Sau save → refreshCallCenterActive() để cập nhật signal
  - initCall — config_ext có thể là JSON string
- FE — `popup-call.service.ts` + `popup-call.ts`
  - Hotline selector
  - popup-call.html — dropdown đầu số
  - Omicall SDK v3 — sipRealm per extension
  - Omicall SDK v3 — audio output (CRITICAL)
  - Omicall SDK v3 — `sdk.init()` không có option `remoteAudio`
  - ⚠️ Omicall register event — KHÔNG dùng `if (data.status)`
  - hangUp — dùng `decline()` cho gọi vào đang ringing
  - Pattern tích hợp vào list component (click SĐT → gọi)
  - makeCall — sipNumber bắt buộc với Omicall
  - Provider form — giữ config_ext khi update
  - Browser permissions — microphone + notification
  - hasCallCenter — Omicall check sipRealm

---


FE patterns cho module Call Center. BE (config, proxy, webhook, masking) → Xem `skill/skills/working-on-config/SKILL.md`.

## FE — `call-center-provider.ts`

**updateRecord** phải fetch full record (list SELECT không có `config_system`):

```typescript
override updateRecord(model: any) {
  this.modelService.get(model.id).subscribe({
    next: (full: any) => {
      const dialogRef = this.dialogWrapperService.open(this.formComponent, {
        data: { model: { ...full, status: model.status } },
      });
      // ...
    },
  });
}
```

**activateRow** phải gọi `resetPage()` (không chỉ updateRow) vì BE deactivate tất cả row khác — nếu chỉ updateRow thì các row cũ vẫn hiển thị "Đã kích hoạt":

```typescript
activateRow(row: any): void {
  this.modelService.activeCallCenter({ id: row.id }).subscribe({
    next: () => {
      this.noti.successMsg(...);
      this.resetPage(); // reload toàn list
    },
  });
}
```

**company_id init trong buildForm** — edit phải preserve NULL (shared template):

```typescript
company_id: new FormControl(
  m.id != null ? (m.company_id ?? null) : (authCompanyActive()?.id ?? null),
  Validators.required,
),
```

---

## FE — `call-center-provider-form.ts`

### Omicall config_system structure

```typescript
config_system = {
  api_key: '...',     // REST API key (server-side, masked)
  sipRealm: '...',    // SIP domain (VD: "demo01") — KHÔNG mask
  hotlines: ['0592041376', '0592041942'], // đầu số gọi ra — KHÔNG mask
}
```

### Hotlines FormArray

Form dùng `FormArray` (`Omicall_Hotlines`) để quản lý nhiều đầu số:

```typescript
get omicallHotlineControls(): FormControl[] {
  const arr = this.rfModel?.get('Omicall_Hotlines') as FormArray;
  return arr ? ([...arr.controls] as FormControl[]) : []; // new ref mỗi lần
}

addHotline() {
  (this.rfModel.get('Omicall_Hotlines') as FormArray).push(this.fb.control(''));
  this.cdr.markForCheck();
}
removeHotline(i: number) {
  (this.rfModel.get('Omicall_Hotlines') as FormArray).removeAt(i);
  this.cdr.markForCheck();
}
```

**patchCredentials** (khi edit):
```typescript
const hoArr = this.rfModel.get('Omicall_Hotlines') as FormArray;
hoArr.clear();
(cs.hotlines ?? []).forEach((h: string) => hoArr.push(this.fb.control(h ?? '')));
if (hoArr.length === 0) hoArr.push(this.fb.control(''));
```

**buildPayload**:
```typescript
const hotlines = ((v.Omicall_Hotlines ?? []) as string[]).filter(h => h?.trim());
config_system = {
  api_key: v.Omicall_ApiKey,
  sipRealm: v.Omicall_SipRealm,
  hotlines,
  ...(v.Omicall_BaseURL?.trim() ? { base_url: v.Omicall_BaseURL.trim() } : {}),
};
```

HTML dùng `formArrayName="Omicall_Hotlines"` + `[formControlName]="i"`.

**base_url field** — optional, để trống = dùng server mặc định:
```typescript
// buildForm — thêm vào group
Omicall_BaseURL: new FormControl<string | null>(null),

// patchCredentials
Omicall_BaseURL: cs.base_url ?? null,

// provider change reset
Omicall_BaseURL: null,
```

---

## FE — `call-center-extensions.ts`

### Omicall extensions dropdown

FE gọi `GET /config-call-center/omicall/extensions` → BE proxy Omicall API, trả `[{sip_user, full_name, email}]`. User chọn từ dropdown, password được BE tự resolve khi save.

```typescript
readonly omicallExtList = signal<OmicallExt[]>([]);
readonly loadingOmicallExts = signal(false);
private omicallExtSub?: Subscription;

loadOmicallExtensions(): void {
  this.omicallExtSub?.unsubscribe();  // cancel request cũ (race condition)
  this.loadingOmicallExts.set(true);
  this.omicallExtSub = this.modelService.getOmicallExtensions().pipe(
    takeUntilDestroyed(this.destroyRef),  // tránh callback sau destroy
    finalize(() => { this.loadingOmicallExts.set(false); this.cdr.markForCheck(); }),
  ).subscribe({
    next: (res: any) => {
      this.omicallExtList.set(Array.isArray(res?.data) ? res.data : []);
      this.cdr.markForCheck();
    },
  });
}
```

**Auto-load khi vào trang** — gọi trong `loadVendor()`:
```typescript
if (this.isOmicall && this.idVendor) {
  this.loadOmicallExtensions();
} else {
  this.omicallExtList.set([]);
}
```

**HTML** — ext column dùng `p-select` thay input khi Omicall:
```html
@if (isOmicall) {
  <p-select formControlName="ext" [options]="omicallExtList()"
    optionValue="sip_user" [filter]="true" filterBy="sip_user,full_name,email">
  </p-select>
} @else {
  <input [formControlName]="isStringee ? 'phonenumber' : 'ext'" pInputText />
}
```

Password column ẩn khi Omicall (`@if (!isOmicall)`). `colspan` điều chỉnh: `[attr.colspan]="isOmicall ? 4 : 5"`.

### extControls getter — PHẢI trả new reference

```typescript
// ĐÚNG: trả [...controls] để p-table (OnPush) detect thay đổi
get extControls(): FormGroup[] {
  const arr = this.rfModel?.get('config_ext') as FormArray;
  return arr ? ([...arr.controls] as FormGroup[]) : [];
}
// SAI: return arr.controls — cùng reference, p-table không update
```

### Luôn có error handler cho API calls

```typescript
this.modelService.getCallCenterActive().subscribe({
  next: ...,
  error: () => {
    this.initialForm({ id: 0, status: 0, provider_type: 0, config_system: {}, config_ext: [] });
    this.noti.errorMsg(...);
    this.cdr.markForCheck();
  },
});
```

Khi `getConfigByProviderType` fail → reset `this.idVendor = 0` (tránh UI inconsistent với Save button enabled nhưng data chưa load).

### onSubmit gửi config_system masked → an toàn

`loadVendor` lưu `this.configSystem` từ GET response (đã masked). Khi submit gửi lại masked value → BE `mergeCredentials` tự restore giá trị gốc. FE không cần xử lý gì thêm.

### Sau save → refreshCallCenterActive() để cập nhật signal

```typescript
call.pipe(finalize(() => this.isSubmitting.set(false))).subscribe({
  next: () => {
    this.noti.successMsg(...);
    void this.authService.refreshCallCenterActive(); // ← cập nhật authUserCallCenter signal
  },
});
```

Nếu không gọi, `popup-call.ts` effect không re-init SDK với data mới (sipRealm, userId mapping).

### initCall — config_ext có thể là JSON string

IndexedDB / signal đôi khi trả `config_ext` dạng string thay vì array. Parse defensive:

```typescript
let config_ext: any[];
if (Array.isArray(callCenter.config_ext)) {
  config_ext = callCenter.config_ext;
} else if (typeof callCenter.config_ext === 'string') {
  try { config_ext = JSON.parse(callCenter.config_ext); } catch { return; }
} else { return; }
if (!Array.isArray(config_ext)) return;
```

---

## FE — `popup-call.service.ts` + `popup-call.ts`

### Hotline selector

```typescript
// Signals trong PopupCallService
readonly hotlines = signal<string[]>([]);
readonly selectedHotline = signal<string | null>(null);

// Set trong initMetaData
initMetaData(systemConfig: any, userExt: any) {
  this.metaData = { ext: userExt?.ext, phonenumber: userExt?.phonenumber, ...systemConfig };
  const lines = (systemConfig?.hotlines ?? []).filter((h: string) => !!h?.trim());
  this.hotlines.set(lines);
  this.selectedHotline.set(lines[0] ?? null); // auto-chọn đầu số đầu tiên
}

// makeCall truyền sipNumber — guard !hotline, KHÔNG bao giờ truyền undefined
// (xem "makeCall — sipNumber bắt buộc với Omicall" bên dưới)
const hotline = this.selectedHotline();
if (!hotline) { /* errorMsg + return */ }
OMICallSDK.makeCall(phone, {
  sipNumber: { number: hotline },
  remoteContact: { name: ... },
});

// unregister reset
this.hotlines.set([]);
this.selectedHotline.set(null);
```

### popup-call.html — dropdown đầu số

```html
@if (popupCallService.hotlines().length > 1) {
  <p-select
    [ngModel]="popupCallService.selectedHotline()"
    (ngModelChange)="popupCallService.selectedHotline.set($event)"
    [options]="popupCallService.hotlines()"
    appendTo="body"
  ></p-select>
} @else {
  <span>{{ popupCallService.hotlines()[0] || extPhone() }}</span>
}
```

popup-call.ts cần import `Select` từ `primeng/select`.

### Omicall SDK v3 — sipRealm per extension

`sipRealm` lưu **per extension** trong `config_ext[].sipRealm`, KHÔNG dùng `config_system.sipRealm` làm primary. SDK register:

```typescript
await sdk.register({
  sipRealm: userExt.sipRealm || callCenter.config_system?.sipRealm, // extension trước, fallback system
  sipUser: userExt.ext,
  sipPassword: userExt.password,
});
```

### Omicall SDK v3 — audio output (CRITICAL)

SDK cần 2 `<video>` elements với id `${callData.uid}-local` / `${callData.uid}-remote` để attach stream. **Tạo dynamically khi call start, xóa khi ended.** SDK không tự attach stream cho custom UI — phải gán `srcObject` thủ công ở `accepted`:

```typescript
// connecting / ringing → mount
mountOmicallVideoElements(callData.uid) {
  for (const side of ['local', 'remote']) {
    const el = document.createElement('video');
    el.id = `${uid}-${side}`;
    el.autoplay = true; el.setAttribute('playsinline', '');
    el.style.display = 'none';
    if (side === 'local') el.muted = true;
    document.body.appendChild(el);
  }
}

// accepted → attach stream
sdk.on('accepted', (callData) => {
  const remoteEl = document.getElementById(`${callData?.uid}-remote`) as HTMLVideoElement;
  if (remoteEl && callData?.streams?.remote) {
    remoteEl.srcObject = callData.streams.remote;
    void remoteEl.play().catch(() => {});
  }
});

// ended + unregister → unmount
unmountOmicallVideoElements() { /* getElementById + remove() */ }
unregister() {
  if (this.activeProvider === 'omicall') {
    OMICallSDK?.unregister?.();
    this.unmountOmicallVideoElements(); // ← PHẢI gọi để tránh DOM leak
  }
}
```

`callData.remoteNumber` (KHÔNG phải `phoneNumber`) khi nhận cuộc gọi vào (`ringing` event).

### Omicall SDK v3 — `sdk.init()` không có option `remoteAudio`

`SdkInitConfigs` chỉ có `lng`, `ui`, `ringtoneVolume`, `sounds`, v.v. — **không có `remoteAudio`**. Audio được handle qua video elements với id `${uid}-remote`.

### ⚠️ Omicall register event — KHÔNG dùng `if (data.status)`

`RegisterData.status` là string `'connecting'|'connected'|'disconnect'` — tất cả đều truthy.

```typescript
// SAI — 'connecting' và 'disconnect' đều set allowUsingCall = true
sdk.on('register', (data) => {
  if (data.status) { this.allowUsingCall.set(true); }
});

// ĐÚNG
sdk.on('register', (data) => {
  if (data.status === 'connected') {
    this.callStatus.set('register_success');
    this.allowUsingCall.set(true);
  } else if (data.status === 'disconnect') {
    this.callStatus.set('register_fail');
    this.allowUsingCall.set(false);
  } else {
    // 'connecting' — chưa cho gọi
    this.callStatus.set('registering');
  }
});
```

### hangUp — dùng `decline()` cho gọi vào đang ringing

`callStatus() === 'incomingcall'` → `omicallData.decline()`; còn lại (gọi ra, đang đàm thoại) → `omicallData.end()`. Khác biệt decline() vs end() → Xem `skill/skills/working-on-config/reference/omicall-sdk.md` (CallData methods).

### Pattern tích hợp vào list component (click SĐT → gọi)

```typescript
// Inject PopupCallService, thêm method:
onPhoneClick(phone: string): void {
  if (!phone) return;
  if (this.popupCallService.allowUsingCall()) {
    this.popupCallService.openPopup(phone, true); // autoCall = true
  } else {
    navigator.clipboard.writeText(phone).then(() => this.noti.success('Đã sao chép', phone));
  }
}
```

Template — bỏ `[appCopyText]`, thay bằng:
```html
<div
  class="cursor-pointer"
  [innerHTML]="model.phone"
  [pTooltip]="(popupCallService.allowUsingCall() ? 'common.tooltip.call_phone' : 'common.tooltip.copy_phone') | translate"
  tooltipPosition="top"
  (click)="onPhoneClick(model.phone)"
></div>
```

i18n key cần thêm: `common.tooltip.call_phone`.

### makeCall — sipNumber bắt buộc với Omicall

Omicall yêu cầu `sipNumber` để biết gọi từ hotline nào. Khi `selectedHotline()` = null (chưa cấu hình hotlines), SDK timeout với lỗi "thuê bao đang bận" thay vì lỗi rõ ràng.

```typescript
makeCall(phone: string): void {
  if (this.activeProvider === 'omicall') {
    const hotline = this.selectedHotline();
    if (!hotline) {
      this.noti.errorMsg(this.translate.instant('utilities.popup_call.no_hotline_configured'));
      return; // KHÔNG để SDK gọi với sipNumber: undefined
    }
    (window as any).OMICallSDK.makeCall(phone, {
      sipNumber: { number: hotline }, // luôn truyền giá trị thực
      remoteContact: { name: this.matchedCustomer()?.fullname || phone },
    });
    return;
  }
}
```

`hotlines` lưu trong `config_system.hotlines: string[]` khi lưu provider config. `initMetaData()` auto-select `hotlines[0]` làm default.

### Provider form — giữ config_ext khi update

`buildPayload()` trong `call-center-provider-form.ts` KHÔNG được hardcode `config_ext: []` khi edit — sẽ xóa toàn bộ máy nhánh đã cấu hình.

```typescript
// SAI — xóa hết extensions khi update provider config
config_ext: [],

// ĐÚNG — giữ nguyên extensions từ model hiện tại
config_ext: this.model()?.config_ext ?? [],
```

`config_ext` (danh sách máy nhánh) được quản lý riêng bởi `CallCenterExtensions` form — provider form chỉ được chạm vào `config_system`.

### Browser permissions — microphone + notification

Gọi `requestCallPermissions()` ở đầu `initProvider()` trước khi load SDK script:

```typescript
private async requestCallPermissions(): Promise<void> {
  // Microphone — bắt buộc để gọi/nghe
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    stream.getTracks().forEach(t => t.stop()); // chỉ cần grant, không giữ stream
  } catch {
    this.noti.warnMsg(this.translate.instant('utilities.popup_call.permission_microphone_denied'));
  }
  // Notification — chỉ hỏi khi còn 'default', không hỏi lại nếu đã grant/deny
  if ('Notification' in window && Notification.permission === 'default') {
    await Notification.requestPermission();
  }
}
```

- Nếu user từ chối microphone → warn nhưng vẫn init SDK (browser sẽ block mic khi thực sự gọi)
- Notification dùng để nhận incoming call khi tab không focus
- Không có permission riêng cho audio output (speaker) — browser cho phép tự động sau khi có user interaction

### hasCallCenter — Omicall check sipRealm

```typescript
// config-call-service.service.ts
hasCallCenter(data: any): boolean {
  if (!data) return false;
  if (data.provider_type === this.VOIP24H)  return !!data.config_system?.SipRealm;
  if (data.provider_type === this.OMICALL)  return !!data.config_system?.sipRealm;
  return !!(data.config_system?.API_SID && data.config_system?.Secret_Key); // Stringee
}
```
