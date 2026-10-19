// Device fingerprint — SHA-256 base64url của các thuộc tính browser tương đối ổn định.
// KHÔNG gồm screen/devicePixelRatio: đổi khi cắm màn hình ngoài → sai fingerprint oan.
// Dùng cho device binding (login + refresh). Đây là defense-in-depth, không phải
// định danh cứng (XSS có thể giả) — kết hợp với HttpOnly cookie + revocation.

let cached: string | null = null;

function toBase64Utf8(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  const CHUNK = 0x8000;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    bin += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(bin);
}

/** Tính fingerprint (cache in-memory — không tính lại mỗi lần refresh token). */
export async function getDeviceFingerprint(): Promise<string> {
  if (cached) return cached;

  const nav = window.navigator as any;
  const raw = JSON.stringify({
    ua: nav.userAgent ?? '',
    lang: nav.language ?? '',
    platform: nav.platform ?? '',
    hw: nav.hardwareConcurrency ?? 0,
    mem: String(nav.deviceMemory ?? 0),
    tz: Intl.DateTimeFormat().resolvedOptions().timeZone ?? '',
  });

  if (globalThis.crypto?.subtle) {
    const bytes = new TextEncoder().encode(raw);
    const buf = await crypto.subtle.digest('SHA-256', bytes);
    cached = btoa(String.fromCharCode(...new Uint8Array(buf)))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/g, '')
      .slice(0, 44);
    return cached;
  }

  cached = toBase64Utf8(raw).slice(0, 44);
  return cached;
}
