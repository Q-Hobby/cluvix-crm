#!/usr/bin/env node
/**
 * i18n apply — Phase 3+4+5.
 *
 * Áp dụng i18n-dedup/mapping.json theo tier:
 *   node scripts/i18n-apply.mjs <safe|risky|all> [--step=add|rewrite|prune|all]
 *
 *   add    : thêm canonical entry mới vào 4 locale (additive, không xoá gì)
 *   rewrite: rewrite reference oldKey→canonical trong .ts/.html (literal quoted)
 *   prune  : xoá oldKey path khỏi 4 locale + dọn parent rỗng
 *   all    : add → rewrite → prune (mặc định)
 *
 * An toàn: rewrite chỉ thay token có dấu nháy bao quanh ('X' "X" `X`) → không đụng
 * key dài hơn. Protected prefix không bao giờ nằm trong remap (đã lọc ở Phase 2).
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');
const I18N_DIR = path.join(FE_ROOT, 'src/assets/i18n');
const FE_SRC = path.join(FE_ROOT, 'src/app');
const LOCALES = ['vi-VN', 'en-US', 'ja-JP', 'fil-PH'];
const SKIP_DIRS = new Set(['node_modules', 'dist', '.angular', 'coverage', '__tests__']);

const tier = process.argv[2];
const stepArg = (process.argv.find(a => a.startsWith('--step=')) || '--step=all').split('=')[1];
if (!['safe', 'risky', 'all'].includes(tier)) { console.error('Usage: i18n-apply.mjs <safe|risky|all> [--step=...]'); process.exit(1); }

function setKey(obj, dotted, value) {
  const parts = dotted.split('.'); let o = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    if (o[parts[i]] == null) o[parts[i]] = {};
    else if (typeof o[parts[i]] !== 'object') throw new Error(`CONFLICT add ${dotted}: '${parts[i]}' is a leaf`);
    o = o[parts[i]];
  }
  o[parts[parts.length - 1]] = value;
}
function delKey(obj, dotted) {
  const parts = dotted.split('.'), stack = []; let o = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    if (o == null || typeof o !== 'object') return false;
    stack.push([o, parts[i]]); o = o[parts[i]];
  }
  if (o == null || typeof o !== 'object') return false;
  if (!(parts[parts.length - 1] in o)) return false;
  delete o[parts[parts.length - 1]];
  for (let i = stack.length - 1; i >= 0; i--) {
    const [par, k] = stack[i];
    if (par[k] && typeof par[k] === 'object' && Object.keys(par[k]).length === 0) delete par[k];
    else break;
  }
  return true;
}

async function* walk(dir) {
  for (const e of await fs.readdir(dir, { withFileTypes: true })) {
    if (e.name.startsWith('.') || SKIP_DIRS.has(e.name)) continue;
    const full = path.join(dir, e.name);
    if (e.isDirectory()) yield* walk(full);
    else if (['.ts', '.html'].includes(path.extname(e.name)) && !full.endsWith('.spec.ts')) yield full;
  }
}

(async () => {
  const { canonical, remap } = JSON.parse(await fs.readFile(path.join(FE_ROOT, 'i18n-dedup/mapping.json'), 'utf8'));
  // lọc remap theo tier
  const entries = Object.entries(remap).filter(([, v]) => tier === 'all' || v.tier === tier);
  const oldKeys = entries.map(([o]) => o);
  const targets = new Set(entries.map(([, v]) => v.to));
  const doAdd = ['add', 'all'].includes(stepArg);
  const doRewrite = ['rewrite', 'all'].includes(stepArg);
  const doPrune = ['prune', 'all'].includes(stepArg);
  console.log(`tier=${tier} step=${stepArg} → ${oldKeys.length} oldKeys, ${targets.size} target keys`);

  // --- ADD: chỉ thêm canonical entries MỚI (có trong canonical{}; reuse-existing đã có sẵn) ---
  if (doAdd) {
    const locObj = {};
    for (const l of LOCALES) locObj[l] = JSON.parse(await fs.readFile(path.join(I18N_DIR, `${l}.json`), 'utf8'));
    let added = 0;
    for (const key of targets) {
      const c = canonical[key];
      if (!c) continue; // reuse-existing → đã có trong locale
      const val = {
        'vi-VN': c.vi,
        'en-US': c.en ?? c.vi,
        'ja-JP': c.ja ?? c.en ?? c.vi,
        'fil-PH': c.fil ?? c.en ?? c.vi,
      };
      for (const l of LOCALES) setKey(locObj[l], key, val[l]);
      added++;
    }
    for (const l of LOCALES)
      await fs.writeFile(path.join(I18N_DIR, `${l}.json`), JSON.stringify(locObj[l], null, 2) + '\n');
    console.log(`  add: +${added} canonical keys × 4 locales`);
  }

  // --- REWRITE: literal quoted token thay thế ---
  if (doRewrite) {
    const map = new Map(entries.map(([o, v]) => [o, v.to]));
    let files = 0, hits = 0;
    for await (const f of walk(FE_SRC)) {
      let c = await fs.readFile(f, 'utf8');
      let changed = false;
      for (const [oldK, newK] of map) {
        for (const q of ["'", '"', '`']) {
          const needle = q + oldK + q, repl = q + newK + q;
          if (c.includes(needle)) { const n = c.split(needle).length - 1; c = c.split(needle).join(repl); hits += n; changed = true; }
        }
      }
      if (changed) { await fs.writeFile(f, c); files++; }
    }
    console.log(`  rewrite: ${hits} replacements in ${files} files`);
  }

  // --- PRUNE: xoá oldKeys khỏi 4 locale ---
  if (doPrune) {
    const locObj = {};
    for (const l of LOCALES) locObj[l] = JSON.parse(await fs.readFile(path.join(I18N_DIR, `${l}.json`), 'utf8'));
    let removed = 0;
    for (const key of oldKeys) {
      let any = false;
      for (const l of LOCALES) any = delKey(locObj[l], key) || any;
      if (any) removed++;
    }
    for (const l of LOCALES)
      await fs.writeFile(path.join(I18N_DIR, `${l}.json`), JSON.stringify(locObj[l], null, 2) + '\n');
    console.log(`  prune: removed ${removed} oldKeys from locales`);
  }
})().catch(e => { console.error(e); process.exit(1); });
