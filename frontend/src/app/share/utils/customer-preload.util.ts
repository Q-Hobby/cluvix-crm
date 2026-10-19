/**
 * Cross-tab customer preload bridge.
 *
 * Customer detail mở qua target="_blank" → không truyền được history.state.
 * Source page ghi customer object vào localStorage trước khi click;
 * detail page đọc + consume ngay khi init → paint instant trước khi HTTP về.
 *
 * TTL ngắn để tránh stale data + tự cleanup khi consume.
 */

const KEY_PREFIX = 'customer:preload:';
const TTL_MS = 30_000;

interface PreloadEntry {
  customer: any;
  ts: number;
}

export function writeCustomerPreload(id: number | string, customer: any): void {
  if (!id || !customer) return;
  try {
    localStorage.setItem(
      `${KEY_PREFIX}${id}`,
      JSON.stringify({ customer, ts: Date.now() } satisfies PreloadEntry),
    );
  } catch {
    // localStorage full / disabled → fail silent, không phải critical path
  }
}

export function consumeCustomerPreload(id: number | string): any | null {
  if (!id) return null;
  const key = `${KEY_PREFIX}${id}`;
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    localStorage.removeItem(key);
    const parsed = JSON.parse(raw) as PreloadEntry;
    if (Date.now() - parsed.ts > TTL_MS) return null;
    return parsed.customer;
  } catch {
    return null;
  }
}
