#!/usr/bin/env node
/**
 * i18n duplicate-value gate (pre-commit) — cảnh báo khi commit thêm key MỚI mang
 * value đã tồn tại CÙNG NGHĨA (đồng âm khác nghĩa EN như Payment/Billing → bỏ qua).
 *
 * So sánh vi-VN.json STAGED (index) vs HEAD → chỉ xét key mới thêm trong commit này,
 * KHÔNG ồn ào với ~414 nhóm trùng cũ còn tồn đọng.
 *
 * Mặc định WARN (exit 0). Đặt I18N_DUP_STRICT=1 để BLOCK commit (exit 1).
 *
 * Usage: node scripts/i18n-dup-check.mjs
 */
import { execSync } from 'node:child_process';

const VI = 'src/assets/i18n/vi-VN.json';
const EN = 'src/assets/i18n/en-US.json';
const STRICT = process.env.I18N_DUP_STRICT === '1';

const flat = (o, p = '', out = {}) => {
  for (const [k, v] of Object.entries(o ?? {})) {
    const np = p ? `${p}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flat(v, np, out);
    else out[np] = v;
  }
  return out;
};
const fold = (s) => String(s ?? '').normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D');
const singular = (w) => w.length <= 3 ? w : /ies$/.test(w) ? w.slice(0, -3) + 'y'
  : /(ses|xes|zes|ches|shes)$/.test(w) ? w.slice(0, -2) : /ss$/.test(w) ? w : /s$/.test(w) ? w.slice(0, -1) : w;
const enSig = (en) => en ? fold(en).toLowerCase().replace(/[^a-z0-9\s]/g, ' ').trim().split(/\s+/).map(singular).join(' ') : '∅';
const viNorm = (vi) => String(vi ?? '').toLowerCase().replace(/\s+/g, ' ').trim();
// nghĩa = (vi chuẩn hoá) + (nghĩa EN) → trùng cả 2 mới coi là duplicate
const sigOf = (vi, en) => viNorm(vi) + ' ║ ' + enSig(en);

function readRef(ref, file) {
  try { return flat(JSON.parse(execSync(`git show ${ref}:${file}`, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'] }))); }
  catch { return null; }
}

const stagedVi = readRef('', VI);          // `git show :file` = staged
const stagedEn = readRef('', EN);
if (!stagedVi) process.exit(0);            // vi-VN không nằm trong commit → bỏ qua

const headVi = readRef('HEAD', VI) ?? {};
const headEn = readRef('HEAD', EN) ?? {};

// index nghĩa của các key ĐÃ CÓ ở HEAD *và VẪN CÒN* sau commit này.
// Key bị XOÁ trong chính commit này không được tính — nếu tính, mọi lần gộp
// (nhiều key cũ → 1 key canonical mới) đều bị cảnh báo oan bởi chính key vừa xoá.
const headIndex = new Map();               // sig → [keys]
for (const k of Object.keys(headVi)) {
  if (!(k in stagedVi)) continue;          // key bị xoá trong commit này
  const s = sigOf(headVi[k], headEn?.[k]);
  if (!headIndex.has(s)) headIndex.set(s, []);
  headIndex.get(s).push(k);
}

// key MỚI = có trong staged, chưa có ở HEAD
const newKeys = Object.keys(stagedVi).filter(k => !(k in headVi));
const warnings = [];
const seenNew = new Map();                 // sig → key mới đầu tiên (bắt 2 key mới trùng nhau)
for (const k of newKeys) {
  const vi = stagedVi[k];
  if (typeof vi !== 'string') continue;
  const s = sigOf(vi, stagedEn?.[k]);
  const existing = headIndex.get(s);
  if (existing && existing.length) warnings.push({ key: k, vi, reuse: existing });
  else if (seenNew.has(s)) warnings.push({ key: k, vi, reuse: [seenNew.get(s)] });
  else seenNew.set(s, k);
}

if (!warnings.length) process.exit(0);

const bar = '─'.repeat(64);
console.error(`\n${bar}`);
console.error(`⚠️  i18n: ${warnings.length} key MỚI trùng value CÙNG NGHĨA với key đã có → nên DÙNG LẠI:`);
console.error(bar);
for (const w of warnings) {
  console.error(`  ✗ ${w.key}  = "${w.vi}"`);
  console.error(`      ↳ dùng lại: ${w.reuse.join('  |  ')}`);
}
console.error(bar);
console.error(`  Tra cứu: npm run i18n:find -- "<value>"`);
console.error(`  Đồng âm KHÁC nghĩa (vd Payment vs Billing) sẽ KHÔNG bị cảnh báo.`);
if (STRICT) {
  console.error(`  ❌ I18N_DUP_STRICT=1 → BLOCK commit. Sửa rồi commit lại (hoặc bỏ env để chỉ cảnh báo).\n`);
  process.exit(1);
}
console.error(`  ⓘ Cảnh báo (không chặn). Đặt I18N_DUP_STRICT=1 nếu muốn chặn cứng.\n`);
process.exit(0);
