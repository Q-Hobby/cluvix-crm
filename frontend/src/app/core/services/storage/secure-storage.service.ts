import { Service} from '@angular/core';

@Service()
export class SecureStorageService {
  private async getKey(): Promise<CryptoKey> {
    const seed = navigator.userAgent + '|cluvix-pepper-2026';
    const enc = new TextEncoder().encode(seed);
    const hash = await crypto.subtle.digest('SHA-256', enc);
    return crypto.subtle.importKey('raw', hash, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
  }

  async encrypt(plaintext: string): Promise<string> {
    const key = await this.getKey();
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const data = new TextEncoder().encode(plaintext);
    const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, data);
    const packed = new Uint8Array(iv.length + ct.byteLength);
    packed.set(iv, 0);
    packed.set(new Uint8Array(ct), iv.length);
    return btoa(String.fromCharCode(...packed));
  }

  async decrypt(stored: string): Promise<string | null> {
    try {
      const packed = Uint8Array.from(atob(stored), (c) => c.charCodeAt(0));
      const iv = packed.slice(0, 12);
      const ct = packed.slice(12);
      const key = await this.getKey();
      const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct);
      return new TextDecoder().decode(pt);
    } catch {
      return null;
    }
  }

  /** JWTs always start with 'eyJ' (base64-encoded JSON header). Everything else is treated as ciphertext. */
  isCiphertext(stored: string): boolean {
    return !stored.startsWith('eyJ');
  }
}
