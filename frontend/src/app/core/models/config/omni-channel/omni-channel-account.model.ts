import { Identifiable } from '@/core/interface/identifiable';

// Channel/status literal theo pkg/define/omni_channel.go (backend) — giữ đồng bộ.
// 'zalo' = Official Account (API chính thức). 'zalo_personal' = nick CÁ NHÂN của nhân viên
// (giao thức Zalo Web) — channel key RIÊNG cố ý, để mọi guard chống-ban không bao giờ chạm OA.
export type OmniChannel = 'zalo' | 'zalo_personal' | 'facebook' | 'instagram' | 'telegram' | 'livechat';
export type OmniAccountStatus = 'connected' | 'needs_reconnect' | 'disconnected';
/** direct = OAuth/token trực tiếp (mặc định, cột BE default). personal = liên kết tài khoản
 * Telegram cá nhân qua MTProto/QR login (omnichat-telegram-userbot). */
export type OmniAccountProvider = 'direct' | 'personal';

export interface OmniChannelAccountModel extends Identifiable {
  id?: number;
  branch_id?: number | null;
  channel?: OmniChannel;
  /** provider — mặc định 'direct' (cột BE default). */
  provider?: OmniAccountProvider;
  /** id THẬT của platform (FB numeric...) — dùng dedup. NULL cho kênh không page-based. */
  platform_page_id?: string | null;
  external_account_id?: string;
  display_name?: string;
  avatar_url?: string;
  status?: OmniAccountStatus;
  last_error?: string | null;
  connected_at?: number | null;
  /** Unix tin inbound gần nhất — health badge (AC4). NULL/cũ hơn N giờ = "chưa nhận tin". */
  last_inbound_at?: number | null;
  created_at?: number;
  updated_at?: number;
  has_credentials?: boolean;
  metadata?: unknown;
}

// ---- Meta (Facebook/Instagram) OAuth connect — story-04 (BE Done, đọc handler thật) ----

/** Page ứng viên trả từ pending-pages — KHÔNG kèm token (server-side only). */
export interface MetaPendingPage {
  page_id: string;
  page_name: string;
}

/** 1 page gửi lên khi POST meta/connect. */
export interface MetaConnectPageInput {
  page_id: string;
  branch_id?: number | null;
}

/** Kết quả connect từng page — FE hiển thị page nào thành công/thất bại. */
export interface MetaConnectResult {
  page_id: string;
  connected: boolean;
  error?: string;
}

// ---- Telegram connect — ĐÃ KHỚP BE thật (story-06 Done, QA 2026-07-16) ----
// BE trả account shape chuẩn: username của bot nằm trong display_name (nhất quán
// account mọi kênh), KHÔNG có field bot_username riêng.

export type TelegramAccountModel = OmniChannelAccountModel;

// ---- Telegram liên kết tài khoản cá nhân (MTProto/QR login) — omnichat-telegram-userbot story-04/10 ----
// CHƯA live-verify BE thật (xem story-04 Log) — shape khớp API Contract trong architecture.md.

export type TelegramPersonalQRStatus =
  | 'pending'
  | 'scanned_need_password'
  | 'success'
  | 'expired'
  | 'error';

export interface TelegramPersonalQRInitResult {
  qr_login_token: string;
  qr_url: string;
  expires_at: number;
  status: TelegramPersonalQRStatus;
}

export interface TelegramPersonalQRStatusResult {
  status: TelegramPersonalQRStatus;
  qr_url?: string;
  expires_at?: number;
  error?: string;
  account?: { id: number; display_name: string };
}

// ---- Zalo CÁ NHÂN — zalo-personal-core story-05/06 ----
// Khác Telegram cá nhân: BE trả **ảnh PNG base64** (`qr_image`) chứ không phải URL để tự vẽ QR;
// KHÔNG có bước 2FA; có thêm `declined` (người dùng bấm từ chối trên điện thoại — KHÔNG phải lỗi
// hệ thống) và `session_dead` (QR đã tự sinh lại hết số lần cho phép ⇒ phải tạo phiên MỚI).
export type ZaloPersonalQRStatus =
  | 'pending'
  | 'scanned'
  | 'declined'
  | 'success'
  | 'expired'
  | 'session_dead'
  | 'error';

export interface ZaloPersonalQRInitResult {
  qr_token: string;
  expires_at: number;
}

export interface ZaloPersonalQRStatusResult {
  status: ZaloPersonalQRStatus;
  /** PNG base64 (KHÔNG kèm tiền tố data:) — chưa có ở lượt poll đầu vì BE sinh QR bất đồng bộ. */
  qr_image?: string;
  /** Tên nick, có ngay từ trạng thái `scanned` (trước khi đăng nhập xong). */
  display_name?: string;
  account_id?: number;
}

// ---- Livechat site config — story-07 (BE Done, khớp handler thật) ----
// widget_theme.logo_url/brand_name/subtitle + pre_chat_form.require_message — thương hiệu
// widget v2 (livechat-widget-v2-opensource story-01/02, BE Done). snippet_host/snippet_host_source
// trả kèm response create/update (story-01 AC4) — nguồn "fallback" nghĩa PUBLIC_BASE_URL chưa cấu
// hình đúng, FE cảnh báo (story-05 AC5).

export interface LivechatWidgetTheme {
  primary_color: string;
  position: 'left' | 'right';
  greeting_text?: string;
  offline_text?: string;
  launcher_label?: string;
  /** URL S3 https, optional — rỗng = widget dùng logo mặc định. */
  logo_url?: string;
  /** optional — rỗng = widget ẩn dòng brand_name (BE tự set = tên site khi tạo mới nếu để trống). */
  brand_name?: string;
  /** optional — rỗng = widget ẩn dòng phụ header. */
  subtitle?: string;
  /** ngôn ngữ hiển thị widget — BE default "vi". */
  locale?: 'vi' | 'en';
  /** chế độ màu widget — BE default "auto" (theo hệ điều hành/browser của khách). */
  color_scheme?: 'auto' | 'light' | 'dark';
  /** khoảng cách nút launcher tới mép ngang (px), 0..200 — undefined = widget dùng 20px. */
  launcher_offset_x?: number;
  /** khoảng cách nút launcher tới mép dọc (px), 0..200 — undefined = widget dùng 20px. */
  launcher_offset_y?: number;
}

export interface LivechatPreChatForm {
  enabled: boolean;
  require_name: boolean;
  require_phone: boolean;
  /** bắt buộc nhập lời nhắn — BE default true. */
  require_message: boolean;
  /** vùng số điện thoại pre-chat form — BE default "VN". */
  phone_region?: 'VN' | 'INTL';
}

export interface LivechatConfigPayload {
  name: string;
  allowed_origins: string[];
  widget_theme?: LivechatWidgetTheme;
  pre_chat_form?: LivechatPreChatForm;
  branch_id?: number | null;
}

export interface LivechatAccountModel extends OmniChannelAccountModel {
  site_key?: string;
  snippet?: string;
  /** host dùng để build snippet (data-host + src) — story-01 resolveSnippetHost. */
  snippet_host?: string;
  /** 'public_base_url' | 'fallback' — 'fallback' = PUBLIC_BASE_URL chưa cấu hình/sai định dạng. */
  snippet_host_source?: string;
  allowed_origins?: string[];
  widget_theme?: LivechatWidgetTheme;
  pre_chat_form?: LivechatPreChatForm;
  // ---- Identity verification — livechat-widget-v2-opensource story-04/06 ----
  /** account list (`livechatRow`) trả field này — panel edit đọc để biết trạng thái ban đầu. */
  identity_enabled?: boolean;
  identity_mandatory?: boolean;
  /** 4 ký tự cuối secret PLAINTEXT, rỗng khi chưa bật — KHÔNG bao giờ có secret đầy đủ ở đây. */
  secret_last4?: string;
  identity_rotated_at?: number | null;
}

/** Response POST/PATCH .../identity (enable/rotate/disable/mandatory) — `identity_secret` CHỈ có ở
 * enable/rotate, đúng 1 lần, KHÔNG bao giờ đọc lại được sau đó (BE story-04 handler comment). */
export interface LivechatIdentityResult {
  identity_enabled: boolean;
  /** Plaintext — CHỈ có ở response enable/rotate. Undefined ở disable/PATCH mandatory. */
  identity_secret?: string;
  secret_last4: string;
  mandatory: boolean;
  identity_mandatory: boolean;
  identity_rotated_at: number | null;
}

/**
 * Shape `metadata` của account channel='livechat' trong response LIST
 * (`attachCredentialMeta` → `livechatRow`, BE `config/omni_channel/handler/handler.go`).
 * Cấu hình livechat KHÔNG nằm phẳng trên account row — list nhét toàn bộ vào `metadata`,
 * nên form edit phải đọc qua đây (bug: mở edit thấy trống mọi field ngoài tên/chi nhánh).
 */
export interface LivechatAccountMetadata {
  site_key?: string;
  allowed_origins?: string[] | null;
  widget_theme?: LivechatWidgetTheme | null;
  pre_chat_form?: LivechatPreChatForm | null;
  identity_enabled?: boolean;
  secret_last4?: string;
  identity_mandatory?: boolean;
  identity_rotated_at?: number | null;
}

/**
 * Gộp account row + `metadata` thành model đầy đủ cho form edit livechat.
 * KHÔNG có endpoint GET detail livechat (chỉ POST/PUT) — list đã trả đủ dữ liệu, chỉ cần phẳng hóa.
 */
export function toLivechatAccountModel(account: OmniChannelAccountModel): LivechatAccountModel {
  const meta = (account.metadata ?? {}) as LivechatAccountMetadata;
  return {
    ...account,
    site_key: meta.site_key,
    allowed_origins: meta.allowed_origins ?? [],
    widget_theme: meta.widget_theme ?? undefined,
    pre_chat_form: meta.pre_chat_form ?? undefined,
    identity_enabled: meta.identity_enabled,
    identity_mandatory: meta.identity_mandatory,
    secret_last4: meta.secret_last4,
    identity_rotated_at: meta.identity_rotated_at ?? null,
  };
}

export type LivechatIdentityAction = 'enable' | 'rotate' | 'disable';

// ── Lượt ĐỒNG BỘ dữ liệu kênh về inbox (zalo-inbound-sync story-06) ──────────────────────────────

/** `history` = kéo lịch sử (Zalo OA) · `friends` = đọc danh bạ (Zalo cá nhân). */
export type OmniSyncKind = 'history' | 'friends';

export type OmniSyncStatus = 'running' | 'success' | 'partial' | 'failed';

/**
 * Lý do thất bại — TÁCH khỏi status vì mỗi lý do đòi người dùng làm một việc KHÁC nhau.
 * Gộp hết vào "thất bại" là bắt họ đoán phải bấm gì tiếp.
 */
export type OmniSyncFailReason =
  | 'permission_denied' // OA chưa được cấp quyền đọc lịch sử → đi đăng ký/cấp quyền, KHÔNG phải kết nối lại
  | 'token_invalid' // token hỏng → kết nối lại kênh
  | 'rate_limited' // Zalo chặn tần suất → chờ rồi thử lại
  | 'quota_exceeded' // chạm trần nội bộ trong ngày
  | 'stale' // lượt bị bỏ rơi và đã được dọn — KHÔNG phải sự cố tích hợp
  | 'not_implemented' // cặp (kênh, kind) hợp lệ nhưng engine chưa nối
  | 'internal';

/**
 * Một lượt đồng bộ. Bốn con số (`conversations`/`new_messages`/`dup_messages`/`error_count`) là thứ
 * người vận hành đọc để biết lượt vừa rồi có làm được gì không — FE PHẢI hiển thị đủ cả bốn.
 *
 * `new_messages` là số row MỚI thật sự ghi vào DB (BE đếm ở tầng persist), không phải số tin nhận về
 * từ nhà cung cấp.
 */
export interface OmniSyncRunModel {
  id: number;
  channel: OmniChannel;
  account_id: number;
  kind: OmniSyncKind;
  status: OmniSyncStatus;
  fail_reason: OmniSyncFailReason | null;
  conversations: number;
  new_messages: number;
  dup_messages: number;
  error_count: number;
  /** null = lượt TỰ ĐỘNG (chạy sau khi kết nối kênh); có giá trị = ai bấm nút. */
  triggered_by: number | null;
  /** Unix GIÂY (KHÁC `omi_message.sent_at` vốn là mili). */
  started_at: number;
  finished_at: number | null;
}

export interface OmniSyncStartResult {
  run_id: number;
  kind: OmniSyncKind;
  status: OmniSyncStatus;
}
