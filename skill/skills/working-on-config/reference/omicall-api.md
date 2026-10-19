# Omicall API

## Mục lục

- Auth flow
- Số nội bộ (Extensions)
- Hotlines (Đầu số)
- Nhóm nội bộ (Ring Groups)
- Kịch bản bấm phím (IVR)
- Kịch bản cuộc gọi (Call Scripts)
- File ghi âm (Greetings)
- Lịch sử cuộc gọi
  - Search v3 (DÙNG CÁI NÀY — v2 deprecated 01/04/2026)
  - Chi tiết cuộc gọi
  - Cập nhật note/tag cuộc gọi
  - Báo cáo tổng quan
  - Đánh giá cuộc gọi v3
- Webhooks
  - Đăng ký
  - Hủy
- Webhook data — CDR (sau khi cuộc gọi kết thúc)
- Webhook realtime events (trong khi cuộc gọi diễn ra)

---


Nguồn: https://api.omicall.com/omicall-api/overview
Base URL: `https://public-v1-stg.omicall.com`  ← tên chính thức, KHÔNG phải staging (đã xác nhận qua curl)

Mỗi tenant có thể dùng server khác — lưu `base_url` vào `config_system` để override. Xem `skills/working-on-config/SKILL.md` → Omicall extensions proxy.

## Auth flow

```
API_KEY (per company, không đổi)
  → GET /api/auth?apiKey={key}   ← PHẢI dùng GET + query param
                                    POST với JSON body không được server đọc (trả api_key_is_required)
  → access_token (Bearer, TTL 24h)
  → dùng header: Authorization: Bearer {token}
```

**Response format chung:**
```json
{ "status_code": 9999, "payload": ..., "key_enabled": false, "instance_id": "stg" }
```
`status_code 9999` = thành công, `-9999` = thất bại. `instance_id` là tên server, không phải môi trường.

**Error codes:** `api_key_is_required` (thiếu/POST body), `api_key_invalid` (sai key), `auth_invalid` (sai server/key).

**API v3 mới dùng `x-api-key` header thay vì Bearer token** — xem từng endpoint.

---

## Số nội bộ (Extensions)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/internal_phone/list` | Danh sách (keyword, page, size) |
| GET | `/api/call_center/extensions/detail` | Chi tiết theo `type`+`keyword` (`sip_user`/`user_email`/`usr_uuid`) |
| POST | `/api/call_center/internal_phone/update` | Cập nhật password, call_timeout |
| GET | `/api/call_center/internal_phone/refresh` | Reset về trạng thái ban đầu (`?sip_user=`) |
| PUT | `/api/call_center/internal_phone/status` | Bật/tắt (`enabled=true/false&sip_user=`) |

**List response** — các field quan trọng:
```json
{
  "domain": "omiteam",           // domain tổng đài
  "outbound_proxy": "wss://...", // outbound proxy
  "sip_user": "147",
  "password": "XXXXXX",
  "full_name": "...",
  "agent_id": "xxx",             // id nhân viên bên OMI
  "email": "...",
  "public_number": "84256..."    // đầu số đang sử dụng
}
```

**Detail response** chứa `pbx_account` với đủ thông tin để register SDK:
```json
{
  "extension": "6349",
  "uuid": "tientv11",
  "pbx_account": {
    "display_name": "6349",
    "sip_user": "6349",
    "sip_password": "rvld582289",
    "sip_web_socket_server": "wss://vh.omicrm.com:7443",
    "sip_realm": "contact18",
    "sip_proxy": "<sip:203.0.113.10;transport=udp>",
    "stun_servers": [],
    "transport": ["tcp", "udp", "ws", "wss"],
    "use_opus": true,
    "opus_quality": 1
  }
}
```
→ `sip_realm` = `sipRealm`, `sip_user` = `sipUser`, `sip_password` = `sipPassword` cho `OMICallSDK.register()`.

---

## Hotlines (Đầu số)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/hotline/list` | Đầu số active cho phép gọi ra của 1 extension (`?extension=`) → trả `string[]` |
| GET | `/api/call_center/hotline/search` | Toàn bộ hotline của DN (page, size, keyword) |
| GET | `/api/call_center/hotline/by-phone` | Thông tin 1 hotline (`?hotline=`) |
| POST | `/api/call_center/hotline/update` | Cập nhật config gọi ra/vào, phân quyền |
| POST | `/api/call_center/hotline/extension/update` | Gắn 1 hotline cho 1 extension cụ thể (`hotline`, `user_email`, `directions: ["outbound","inbound"]`) |

**Hotline update body** — các field:
- `hotline`: số hotline
- `extensions`: `["101","200"]` — phân quyền gọi ra theo extension
- `group_ids`: `["id1","id2"]` — phân quyền theo nhóm nội bộ
- `call_script`: ID kịch bản gọi vào mặc định
- `allow_call_in` / `allow_call_out`: bật/tắt chiều gọi
- `access_type`: `"applies_to_all_employees"` (tất cả) hoặc `"applies_according_to_employee_criteria"` (theo phân quyền)

---

## Nhóm nội bộ (Ring Groups)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/internal_group/list` | Danh sách nhóm (keyword, page, size) |
| POST | `/api/call_center/internal_group/add` | Tạo nhóm |
| POST | `/api/call_center/internal_group/update/:id` | Cập nhật nhóm |
| POST | `/api/call_center/internal_group/add-members` | Thêm sip_users vào nhóm (`group_id`, `sip_users[]`) |
| POST | `/api/call_center/internal_group/remove-members` | Xóa sip_users khỏi nhóm (`group_id`, `sip_users[]`) |
| DELETE | `/api/call_center/internal_group/delete/:id` | Xóa nhóm |

**Create/Update body:**
- `group_name*`, `strategy*`, `sip_members*` (array ext), `timeout*` (giây chờ per NV)
- `strategy`: `enterprise` (đồng thời), `rollover` (tuần tự), `random`
- `ring_group_type`: `1` = ưu tiên NV rảnh cao nhất, `4` = NV ít cuộc gọi trả lời nhất trong ngày, `0` = không tiêu chí
- `target_timeout_type` / `target_timeout`: kịch bản chuyển tiếp khi kết thúc nhóm (`Recording`/`Ivr_menu`/`Ring_group` + id)
- `call_queue_config`: `{ enable, is_in_call, is_offline, waiting_time }` — cấu hình hàng đợi

---

## Kịch bản bấm phím (IVR)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/key-interaction/list` | Danh sách (keyword, page, size) |
| POST | `/api/call_center/key-interaction/add` | Tạo mới |
| POST | `/api/call_center/key-interaction/update/:id` | Cập nhật |
| DELETE | `/api/call_center/key-interaction/delete/:id` | Xóa |

**Create/Update body:**
- `script_name*`, `max_failure*` (số lần sai phím), `time_out*` (giây chờ KH bấm)
- `greeting_id*`: ID file ghi âm lời chào; `greeting_short_id`: nhạc chờ khi quay lại; `invalid_sound_id`: nhạc khi sai phím
- `scripts*`: mảng `{ ivr: "1", action_type: "Ring_group"|"Ivr_menu"|"Extension"|"Recording", action: "<id>" }`
- `action_type` + `action`: hành động khi không bấm / bấm sai quá lần cho phép

---

## Kịch bản cuộc gọi (Call Scripts)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/call_script/list` | Danh sách (page, size) |
| POST | `/api/call_center/call_script/add` | Tạo mới |
| PUT | `/api/call_center/call_script/update/:id` | Cập nhật |
| DELETE | `/api/call_center/call_script/delete/:id` | Xóa |

**Create/Update body:**
- `script_name*`, `note`
- `scripts*`: `[{ action_id, action_type }]` — `action_type`: `Extension`/`Ivr_menu`/`Ring_group`/`Recording`
- `criterias`: `[{ type, timeout_time, wait_time }]`
  - `type "2"` = NV quản lý KH; `"3"` = NV từng gọi KH đó; `"4"` = NV có cuộc gọi nhỡ với KH
  - `timeout_time`: giây chờ NV; `wait_time`: giây chờ khi NV không online

---

## File ghi âm (Greetings)

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/call_center/greeting/list` | Danh sách |
| POST | `/api/call_center/greeting/upload` | Upload mp3 (form-data) |
| POST | `/api/call_center/greeting/upload-text-to-speech` | TTS (text < 300 ký tự) |
| DELETE | `/api/call_center/greeting/delete/:id` | Xóa |

TTS `voice` values: `northern_female_ngocanh`, `southern_male_tienhuy`, `northern_male_anhkiet`, `southern_female_khangan`.

---

## Lịch sử cuộc gọi

### Search v3 (DÙNG CÁI NÀY — v2 deprecated 01/04/2026)

```
POST /api/v3/call-transaction/search?page=1&size=50
Header: x-api-key: {API_KEY}
Body: { filter: { fromDate, toDate, directions?, sipUsers?, phoneNumbers?, ... }, sort: { field, isAsc } }
```

Giới hạn: 3 tháng gần nhất, max 50/page.

### Chi tiết cuộc gọi

```
POST /api/v2/callTransaction/getByTransactionId
Body: { transactionId: "uuid" }
```

### Cập nhật note/tag cuộc gọi

```
POST /api/call_transaction/change/:transaction_id
Body: { note?: string, tag?: string[] }
```

### Báo cáo tổng quan

```
GET /api/v3/call-transaction/report?fromDate=&toDate=
```
Trả về thống kê theo hướng gọi, trạng thái, theo ngày. Giới hạn 12 tháng.

### Đánh giá cuộc gọi v3

```
POST /api/v3/call-transaction/add-evaluation
Body: { transactionId, agentId, evaluation: { comment, values: [{ criteriaId, name, point }] } }
```

---

## Webhooks

### Đăng ký

```
POST /api/webhooks/register
Body: { webhook: { type: "call", url: "https://...", events: ["ringing","answered","hangup"] } }
```

### Hủy

```
POST /api/webhooks/destroy?hook_type=call
```

---

## Webhook data — CDR (sau khi cuộc gọi kết thúc)

OMI POST tới URL đã đăng ký. Các field quan trọng:

| Field | Mô tả |
|---|---|
| `transaction_id` | Id cuộc gọi |
| `direction` | `outbound`/`inbound`/`local` |
| `answer_sec` | > 0 = có trả lời, = 0 = không trả lời |
| `phone_number` | Số điện thoại khách hàng |
| `hotline` / `sip_number` | Đầu số |
| `sip_user` | Số máy lẻ nhân viên |
| `time_start_call` | Unix timestamp (× 1000 = ms) |
| `time_end_call` | Unix timestamp (× 1000 = ms) |
| `time_answer_start` | Unix timestamp (× 1000 = ms) |
| `bill_sec` | Giây tính cước |
| `recording_file_url` | URL file ghi âm |
| `create_by.name` | Người tạo cuộc gọi |
| `is_callbot` | true nếu callbot |
| `collect_info_variables` | Mảng biến thu thập của callbot |
| `call_stacks` | Luồng cuộc gọi qua từng nhánh |
| `users_status` | Trạng thái NV khi có cuộc gọi (is_online, is_busy, is_dnd, ring_at, answer_at) |

**3 gotcha xử lý CDR** (dedup theo `transaction_id` — OMI gửi nhiều lần; `sip_user` ở ROOT payload KHÔNG phải `create_by`; guard `create_by.name = 'create_default_by_tenant'`).

## Webhook realtime events (trong khi cuộc gọi diễn ra)

States: `create`, `early`, `ringing`, `answered`, `hangup`.

Mỗi cuộc gọi có **2 legs** (KH + NV) — webhook bắn trên mỗi leg.

```json
{
  "call_uuid": "uuid",
  "state": "ringing",
  "direction": "outbound",
  "from_number": "2022",     // sip_user NV (gọi ra) / sđt KH (gọi vào)
  "to_number": "0966...",    // sđt KH (gọi ra) / sip_user NV (gọi vào)
  "phone_number": "0966...", // sđt KH
  "hotline": "02873022488",
  "extension": "2022"
}
```

**Lưu ý gọi vào:** `to_number` có thể rỗng (lời chào, hộp thư thoại, thiết bị offline), hoặc là group_number, script_number, extension tùy kịch bản.
