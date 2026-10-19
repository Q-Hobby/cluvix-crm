/**
 * perm-define-parse — parser dùng chung cho permission.define.ts.
 * Tách từ perm-audit.mjs để perm-codegen.mjs tái dùng (KHÔNG copy-paste).
 *
 * Export:
 *   sliceBalanced(text, startIdx, open, close)
 *   parseSelect(text)  → { map, valueToConst, constToApp }
 *   parseMatrix(text, selectMap) → mảng module PERMISSION_MATRIX (value đã resolve về string)
 */

// ── Bracket-aware slice: lấy text trong cặp mở/đóng, bỏ qua string literal ────────
export function sliceBalanced(text, startIdx, open, close) {
  let depth = 0, i = startIdx, quote = null, out = '';
  for (; i < text.length; i++) {
    const c = text[i];
    if (quote) {
      out += c;
      if (c === quote && text[i - 1] !== '\\') quote = null;
      continue;
    }
    if (c === '"' || c === "'" || c === '`') { quote = c; out += c; continue; }
    if (c === open) depth++;
    if (c === close) depth--;
    out += c;
    if (depth === 0 && (c === open || c === close)) break;
  }
  return out;
}

// ── Parse PERMISSION_SELECT → SELECT_MAP(constKey→value) + section(appModule) ─────
export function parseSelect(text) {
  const start = text.indexOf('export const PERMISSION_SELECT');
  const braceStart = text.indexOf('{', start);
  const block = sliceBalanced(text, braceStart, '{', '}');
  const lines = block.split('\n');
  const map = {};            // constKey -> value
  const valueToConst = {};   // value -> constKey
  const constToApp = {};     // constKey -> appModule
  let section = 'Unknown';
  const secRe = /\/\/\s*─+\s*([^─—]+?)(?:\s*—|\s*─)/;
  const kvRe = /^\s*(permission\w+)\s*:\s*'([^']+)'/;
  for (const line of lines) {
    const sm = line.match(secRe);
    if (sm) { section = sm[1].trim(); continue; }
    const km = line.match(kvRe);
    if (km) {
      const [, k, v] = km;
      map[k] = v;
      valueToConst[v] = k;
      constToApp[k] = section;
    }
  }
  return { map, valueToConst, constToApp };
}

// ── Parse PERMISSION_MATRIX → mảng module (eval sau khi thay PERMISSION_SELECT.X) ──
export function parseMatrix(text, selectMap) {
  const start = text.indexOf('export const PERMISSION_MATRIX');
  const eq = text.indexOf('=', start);              // bỏ qua `[]` trong `PermissionMatrixModule[] =`
  const arrStart = text.indexOf('[', eq);
  let block = sliceBalanced(text, arrStart, '[', ']');
  // thay mọi PERMISSION_SELECT.X bằng chuỗi value (hoặc marker badref nếu const không tồn tại)
  block = block.replace(/PERMISSION_SELECT\.(\w+)/g, (_, k) =>
    JSON.stringify(k in selectMap ? selectMap[k] : `__BADREF__:${k}`));
  // eslint-disable-next-line no-new-func
  const arr = Function(`"use strict"; return (${block});`)();
  return arr;
}
