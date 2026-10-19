# Omicall SDK

## Mục lục

- Cài đặt
- Init
- Register (kết nối tổng đài)
- ⚠️ CRITICAL — sipPassword là per-extension
- Config mapping ERP → Omicall
- Methods
- Events
- OMICallSDK là global singleton — đăng ký listener MỘT LẦN
- CallData — đầy đủ
- Custom UI — bắt buộc có 2 video element

---


Nguồn: https://api.omicall.com/sdk/web-sdk/v3-integration
Version mới nhất: **3.0.42** (2026-05-14) — CDN only, NPM coming soon.

## Cài đặt

```html
<body>
  <script
    omi-call-sdk
    type="text/javascript"
    src="https://cdn.omicrm.com/sdk/web/{{version}}/core.min.js"
  ></script>
</body>
```

Sau khi load: 2 global object `OMICallSDK` (core) + `OMICallUI` (default UI).

**Production bắt buộc HTTPS** — trình duyệt chỉ mở mic/sound trên HTTPS.

## Init

```typescript
const initSuccess: boolean = await OMICallSDK.init({
  lng?: 'vi' | 'en' | 'km',                    // default: 'vi'
  ui?: {
    toggleDial?: 'show' | 'hide',               // default: 'show'
    dialPosition?: 'left' | 'right',            // default: 'right'
    minimizeNewCall?: boolean,                  // default: false
    theme?: { primary, info, success, ... },
  },
  ringtoneVolume?: number,                      // 0-1, default: 1
  phoneMasked?: { enable, len, char, ltr },
  ctaButtons?: {
    unknown?: CtaButton[],
    exists?: CtaButton[],
    base?: CtaButton[],
    onClick?: (callData) => void,
  },
  callTabs?: CallTab[],                         // thay tab "Thông tin cuộc gọi"
  rootBody?: HTMLElement,                       // cho Angular/React HOC
  searchRemoteContact?: (callData) => Promise<RemoteContact | null>,
  validateCallOut?: (number, options) => Promise<{ status: boolean; reason?: string }>,
  icons: string[],
  sounds?: Record<'ringtone'|'ringing'|'accepted'|'ended', { src; volume }>,
  searchRecentCall?: boolean,
});
if (!initSuccess) return; // nếu lỗi init → KHÔNG register
```

## Register (kết nối tổng đài)

```typescript
const result = await OMICallSDK.register({
  sipRealm: string,      // domain tổng đài — per-extension config_ext[userId].sipRealm, fallback config_system.sipRealm
  sipUser: string,       // số nhánh ext — từ config_ext[userId].ext
  sipPassword: string,   // password nhánh đó — từ config_ext[userId].password
  isGuest?: boolean,     // v3.0.35+: skip auth, connect SIP bất kỳ
});
// result: { status: boolean; message?: string; error?: string }

OMICallSDK.unregister();
```

## ⚠️ CRITICAL — sipPassword là per-extension

```typescript
// ĐÚNG
await OMICallSDK.register({
  sipRealm: userExt.sipRealm || callCenter.config_system?.sipRealm, // per-extension trước, fallback system
  sipUser:  userExt.ext,
  sipPassword: userExt.password,   // ← từ config_ext, KHÔNG phải config_system
});

// SAI — config_system.sipPassword không tồn tại
await OMICallSDK.register({ sipPassword: callCenter.config_system.sipPassword });
```

## Config mapping ERP → Omicall

| ERP field | Omicall param | Ghi chú |
|---|---|---|
| `config_ext[userId].sipRealm` | `register.sipRealm` | Domain tổng đài per-extension — primary |
| `config_system.sipRealm` | `register.sipRealm` | Fallback khi extension chưa có `sipRealm` |
| `config_system.api_key` | — | Server-side only, không dùng trong WebRTC SDK |
| `config_system.hotlines` | `makeCall sipNumber.number` | Đầu số gọi ra — user chọn dropdown |
| `config_ext[userId].ext` | `register.sipUser` | Số nhánh |
| `config_ext[userId].password` | `register.sipPassword` | Password per-extension |
| `provider_type = 3` | Omicall | `define.CallCenterProviderOmicall` |

## Methods

```typescript
// Gọi ra
OMICallSDK.makeCall(remoteNumber: string, options?: {
  isVideo?: boolean,
  sipNumber?: { number: string },        // đầu số gọi ra
  remoteContact?: RemoteContact,
  userData?: string,
});

// Click-2-call (gọi từ IP Phone/Softphone)
OMICallSDK.remoteCall(remoteNumber: string, sipNumber?: string);

// Default UI — mở/đóng dial pad
OMICallUI.toggleDial();
```

**Gọi với chọn đầu số** — `sipNumber` bắt buộc, KHÔNG bao giờ truyền `undefined`:
```typescript
const hotline = this.selectedHotline(); // signal từ PopupCallService
if (!hotline) {
  this.noti.errorMsg(...); // báo lỗi rõ ràng, KHÔNG để SDK gọi với sipNumber: undefined
  return;
}
OMICallSDK.makeCall(remoteNumber, {
  sipNumber: { number: hotline },
  remoteContact: { name, avatar?, gender? },
});
```

## Events

```typescript
// Một eventName hỗ trợ nhiều callback → khi off() PHẢI truyền đúng ref đã đăng ký
OMICallSDK.on(eventName, callback);
OMICallSDK.off(eventName, callback);  // ✅ CÓ off() — KHÔNG phải "không có off()"

type EventName =
  | 'register'       // { status: 'connecting'|'connected'|'disconnect'; name }
  | 'connecting'     // cuộc gọi ra bắt đầu kết nối → cache CallData
  | 'ringing'        // cuộc gọi đến đổ chuông → cache CallData
  | 'on_ringing'     // mỗi giây khi đang đổ chuông
  | 'accepted'       // bắt máy
  | 'on_calling'     // mỗi giây khi đang đàm thoại
  | 'set_sip_number' // khi có được hotline đang dùng để gọi ra
  | 'ended'          // cuộc gọi kết thúc
```

## OMICallSDK là global singleton — đăng ký listener MỘT LẦN

```typescript
if (!this.omicallListenersRegistered) {
  this.omicallListenersRegistered = true;
  OMICallSDK.on('register', ...);
  OMICallSDK.on('ringing', ...);
  // ... các event khác
}
await OMICallSDK.register({ sipRealm, sipUser, sipPassword });
```

## CallData — đầy đủ

```typescript
type CallData = {
  uid: string;           // UUID tự sinh — dùng làm id cho <video> elements
  uuid?: string;         // UUID từ tổng đài
  state: 'connecting' | 'ringing' | 'accepted' | 'ended';
  direction: 'outbound' | 'inbound';
  isVideo: boolean;
  isOutbound: boolean;
  isInternal: boolean;
  isHangup?: boolean;    // user chủ động từ chối
  sipNumber?: { number: string };
  remoteContact?: RemoteContact;
  remoteNumber: string;
  displayNumber: string; // theo phân quyền ẩn sđt
  streams: Record<'local' | 'remote', MediaStream>;
  players: Record<'local' | 'remote', HTMLVideoElement>;
  startTs: number;       // timestamp bắt đầu
  startAt: string;       // HH:mm:ss
  held: boolean;
  audio: boolean;        // mic on/off
  video: boolean;        // camera on/off
  ua: string;
  devices: { audio: boolean; video: boolean };
  rtcpStat: any;
  rejectCode?: string;   // mã lỗi từ tổng đài
  blockedProvider?: string; // nhà mạng từ chối
  ringingDuration: { value: number; text: string };
  callingDuration: { value: number; text: string };
  userData?: string;
  isEnded?: boolean;

  // Methods
  end(): void;           // kết thúc cuộc gọi (chung)
  decline(): void;       // từ chối hoàn toàn, KHÔNG tiếp tục phân bổ (khác end())
  accept(): void;
  mute(cb?: () => boolean): void;
  camera(cb?: () => boolean): void;  // bật/tắt camera
  hold(cb?: () => boolean): void;
  dtmf(tone: string): void;          // mỗi tone là 1 ký tự [0-9*#]
  transfer(target: string): void;
  save(info: { note?: string; tags?: string[] }, onSave: () => void, onFinish: () => boolean): void;
  minimize(getCustomPosition?: (data: { width: number; height: number }) => { x: number; y: number }): void;
};
```

**Cache CallData:**
- Event `connecting` → gọi ra
- Event `ringing` → gọi đến

## Custom UI — bắt buộc có 2 video element

```html
<video id="${callData.uid}-local" style="display:none" playsinline autoplay hidden mute />
<video id="${callData.uid}-remote" style="display:none" playsinline autoplay hidden />
```
