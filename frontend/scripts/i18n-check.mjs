#!/usr/bin/env node
/**
 * i18n key audit — Sprint D tooling.
 *
 * Quét toàn bộ FE source code để tìm các translation key đang được dùng,
 * so sánh với vi-VN.json + en-US.json để phát hiện:
 *   - MISSING:  key dùng trong code nhưng KHÔNG có trong JSON → render raw key
 *   - ORPHAN:   key trong JSON nhưng KHÔNG dùng trong code → cleanup được
 *   - MISMATCH: key chỉ có ở vi-VN hoặc en-US (không có cả 2)
 *   - BE↔FE:    key BE i18n.T() có khớp với FE JSON không (cho keys BE return về client)
 *
 * Usage:
 *   node frontend/scripts/i18n-check.mjs
 *   node frontend/scripts/i18n-check.mjs --json    # output JSON cho CI
 *   node frontend/scripts/i18n-check.mjs --fail-on-missing   # exit 1 nếu có missing key
 *
 * Limitations:
 *   - Dynamic keys không detect được (e.g. `'prefix.' + variable | translate`)
 *   - Computed labelKey từ runtime data → false positive ORPHAN. Filter manually.
 */

import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const FE_ROOT = path.resolve(__dirname, '..');
const REPO_ROOT = path.resolve(FE_ROOT, '..');

const FE_SRC_DIR = path.join(FE_ROOT, 'src/app');
const VI_JSON = path.join(FE_ROOT, 'src/assets/i18n/vi-VN.json');
const EN_JSON = path.join(FE_ROOT, 'src/assets/i18n/en-US.json');
const DYNAMIC_KEYS_FILE = path.join(__dirname, 'i18n-dynamic-keys.json');
const BE_VI_GO = path.join(REPO_ROOT, 'backend/pkg/i18n/vi.go');
const BE_EN_GO = path.join(REPO_ROOT, 'backend/pkg/i18n/en.go');

// Patterns matching translation calls in FE code (TS + HTML).
const KEY_PATTERNS = [
  // Single-quoted: 'key.path' | translate
  /'([a-zA-Z_][\w.-]*)'\s*\|\s*translate/g,
  // Double-quoted: "key.path" | translate
  /"([a-zA-Z_][\w.-]*)"\s*\|\s*translate/g,
  // translate.instant('key')
  /translate(?:Service)?\.instant\(\s*['"]([a-zA-Z_][\w.-]*)['"]\s*[,)]/g,
  // translate.get('key')
  /translate(?:Service)?\.get\(\s*['"]([a-zA-Z_][\w.-]*)['"]\s*[,)]/g,
  // translate.stream('key')
  /translate(?:Service)?\.stream\(\s*['"]([a-zA-Z_][\w.-]*)['"]\s*[,)]/g,
  // i18n-related *Key properties — explicit whitelist (KHÔNG dùng `\w*Key` vì
  // catch nhầm `queryKey:` trong filter-mapping config).
  /\b(?:labelKey|titleKey|breadcrumbKey|descKey|descriptionKey|placeholderKey|tooltipKey|messageKey|nameKey|typeModuleKey|headerKey|subtitleKey)\s*:\s*['"]([a-zA-Z_][\w.-]*)['"]/g,
  // Route data: { breadcrumb: 'KEY', title: 'KEY' } — Angular router config.
  // KHÔNG match `label:` vì đó thường là raw display string (e.g. label: 'AI', 'VND').
  // Match only UPPERCASE_SNAKE keys (legacy style) or dotted lower (new style).
  /\b(?:breadcrumb|title)\s*:\s*['"]([A-Z][A-Z0-9_]+|[a-z][\w]*\.[\w.]+)['"]/g,
];

// File extensions to scan.
const SCAN_EXTS = new Set(['.ts', '.html']);
// Directories to skip.
const SKIP_DIRS = new Set(['node_modules', 'dist', '.angular', 'coverage', '__tests__']);

const args = process.argv.slice(2);
const OUTPUT_JSON = args.includes('--json');
const FAIL_ON_MISSING = args.includes('--fail-on-missing');
const VERBOSE = args.includes('--verbose');

async function* walk(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.name.startsWith('.') && entry.name !== '.amazonq') continue;
    if (SKIP_DIRS.has(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(full);
    else if (SCAN_EXTS.has(path.extname(entry.name))) yield full;
  }
}

async function extractKeysFromFE() {
  const usedKeys = new Map(); // key → Set of files
  let fileCount = 0;
  for await (const file of walk(FE_SRC_DIR)) {
    if (file.endsWith('.spec.ts')) continue; // skip test files
    fileCount++;
    const content = await fs.readFile(file, 'utf-8');
    const rel = path.relative(FE_ROOT, file);
    for (const pattern of KEY_PATTERNS) {
      pattern.lastIndex = 0;
      let m;
      while ((m = pattern.exec(content)) !== null) {
        const key = m[1];
        if (!key || key.length < 2) continue;
        if (!usedKeys.has(key)) usedKeys.set(key, new Set());
        usedKeys.get(key).add(rel);
      }
    }
  }
  return { keys: usedKeys, fileCount };
}

function flattenJson(obj, prefix = '', out = new Set()) {
  for (const [k, v] of Object.entries(obj ?? {})) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flattenJson(v, path, out);
    else out.add(path);
  }
  return out;
}

async function loadJson(file) {
  try {
    const raw = await fs.readFile(file, 'utf-8');
    const obj = JSON.parse(raw);
    return flattenJson(obj);
  } catch (e) {
    console.error(`Failed to read ${file}: ${e.message}`);
    return new Set();
  }
}

async function loadGoKeys(file) {
  try {
    const raw = await fs.readFile(file, 'utf-8');
    const keys = new Set();
    // Match Go map literal: "key": "value",
    const re = /^\s*"([a-zA-Z_][\w.-]*)"\s*:/gm;
    let m;
    while ((m = re.exec(raw)) !== null) keys.add(m[1]);
    return keys;
  } catch (e) {
    console.error(`Failed to read ${file}: ${e.message}`);
    return new Set();
  }
}

// Key BE gửi xuống động — không có literal ở FE
// nên không bao giờ được coi là ORPHAN. Nguồn: scripts/i18n-dynamic-keys.json.
async function loadDynamicAllowlist() {
  const raw = JSON.parse(await fs.readFile(DYNAMIC_KEYS_FILE, 'utf-8'));
  const keys = new Set(raw.keys ?? []);
  const prefixes = raw.prefixes ?? [];
  return (k) => keys.has(k) || prefixes.some((p) => k.startsWith(p));
}

function diff(usedSet, jsonSet, isDynamic = () => false) {
  const missing = [];
  const orphan = [];
  for (const k of usedSet) if (!jsonSet.has(k)) missing.push(k);
  for (const k of jsonSet) if (!usedSet.has(k) && !isDynamic(k)) orphan.push(k);
  return { missing, orphan };
}

(async () => {
  console.log('🔍 Scanning FE source...');
  const { keys: usedMap, fileCount } = await extractKeysFromFE();
  const usedSet = new Set(usedMap.keys());

  console.log('📂 Loading JSON files...');
  const viKeys = await loadJson(VI_JSON);
  const enKeys = await loadJson(EN_JSON);

  console.log('📄 Loading BE Go i18n maps...');
  const beViKeys = await loadGoKeys(BE_VI_GO);
  const beEnKeys = await loadGoKeys(BE_EN_GO);

  const isDynamic = await loadDynamicAllowlist();
  const allowlisted = new Set([...viKeys, ...enKeys].filter(isDynamic)).size;

  // FE diff
  const viDiff = diff(usedSet, viKeys, isDynamic);
  const enDiff = diff(usedSet, enKeys, isDynamic);

  // Cross-language consistency
  const viOnly = [...viKeys].filter(k => !enKeys.has(k));
  const enOnly = [...enKeys].filter(k => !viKeys.has(k));

  // BE intra
  const beViOnly = [...beViKeys].filter(k => !beEnKeys.has(k));
  const beEnOnly = [...beEnKeys].filter(k => !beViKeys.has(k));

  // BE → FE: BE keys used as response messages may need FE counterpart only if FE re-translates.
  // Cluvix BE translates server-side → FE doesn't need BE keys. But if a key starts with `field.`
  // or matches `error.*` patterns, it might be shared. For now, just report intersection.
  const beInFeViMissing = [...beViKeys].filter(k => !viKeys.has(k));

  const report = {
    summary: {
      fe_files_scanned: fileCount,
      fe_keys_used_in_code: usedSet.size,
      vi_VN_total: viKeys.size,
      en_US_total: enKeys.size,
      be_vi_total: beViKeys.size,
      be_en_total: beEnKeys.size,
      dynamic_allowlisted: allowlisted,
    },
    fe_missing_in_vi: viDiff.missing.sort(),
    fe_missing_in_en: enDiff.missing.sort(),
    fe_orphan_in_vi: viDiff.orphan.sort(),
    fe_orphan_in_en: enDiff.orphan.sort(),
    fe_vi_only: viOnly.sort(),
    fe_en_only: enOnly.sort(),
    be_vi_only: beViOnly.sort(),
    be_en_only: beEnOnly.sort(),
    be_keys_not_in_fe_vi: beInFeViMissing.sort(),
  };

  if (OUTPUT_JSON) {
    console.log(JSON.stringify(report, null, 2));
  } else {
    const fmt = (n) => String(n).padStart(5);
    console.log('\n=== i18n Key Audit ===\n');
    console.log(`📊 Summary`);
    console.log(`   FE files scanned:        ${fmt(report.summary.fe_files_scanned)}`);
    console.log(`   FE keys used in code:    ${fmt(report.summary.fe_keys_used_in_code)}`);
    console.log(`   vi-VN.json total:        ${fmt(report.summary.vi_VN_total)}`);
    console.log(`   en-US.json total:        ${fmt(report.summary.en_US_total)}`);
    console.log(`   BE vi.go total:          ${fmt(report.summary.be_vi_total)}`);
    console.log(`   BE en.go total:          ${fmt(report.summary.be_en_total)}`);
    console.log(`   Allowlist động (BE gửi): ${fmt(report.summary.dynamic_allowlisted)} key (scripts/i18n-dynamic-keys.json)`);

    const section = (title, list, severity = '') => {
      console.log(`\n${severity} ${title} (${list.length})`);
      if (list.length === 0) {
        console.log('   ✓ none');
        return;
      }
      const show = VERBOSE ? list : list.slice(0, 20);
      show.forEach(k => {
        const refs = usedMap.get(k);
        const ref = refs ? ` [used in: ${[...refs].slice(0, 2).join(', ')}${refs.size > 2 ? '...' : ''}]` : '';
        console.log(`   - ${k}${ref}`);
      });
      if (!VERBOSE && list.length > 20) console.log(`   ... +${list.length - 20} more (use --verbose)`);
    };

    section('MISSING in vi-VN.json (used in code but missing)', report.fe_missing_in_vi, '❌');
    section('MISSING in en-US.json', report.fe_missing_in_en, '❌');
    section('ORPHAN in vi-VN.json (in JSON but not used)', report.fe_orphan_in_vi, '⚠️ ');
    section('ORPHAN in en-US.json', report.fe_orphan_in_en, '⚠️ ');
    section('vi-VN ONLY (not in en-US)', report.fe_vi_only, '🔀');
    section('en-US ONLY (not in vi-VN)', report.fe_en_only, '🔀');
    section('BE vi.go ONLY (not in BE en.go)', report.be_vi_only, '🔀');
    section('BE en.go ONLY (not in BE vi.go)', report.be_en_only, '🔀');
  }

  const missingTotal = report.fe_missing_in_vi.length + report.fe_missing_in_en.length;
  if (FAIL_ON_MISSING && missingTotal > 0) {
    console.error(`\n❌ ${missingTotal} missing keys → exit 1`);
    process.exit(1);
  }
})().catch((err) => {
  console.error(err);
  process.exit(2);
});
