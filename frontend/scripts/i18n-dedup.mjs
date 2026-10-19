#!/usr/bin/env node
/**
 * i18n dedup analyzer — Phase 0 tooling.
 *
 * Quét 4 locale (vi/en/ja/fil) + FE source code, group key theo value vi-VN,
 * xuất report cho mỗi nhóm trùng: tất cả key path, bản dịch 4 locale per path,
 * usage count + files, cờ dynamic-key.
 *
 * Output:
 *   i18n-dedup/report.json   — máy đọc (cho codemod Phase 4)
 *   i18n-dedup/report.md     — người review
 *
 * Usage: node scripts/i18n-dedup.mjs
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');
const I18N_DIR = path.join(FE_ROOT, 'src/assets/i18n');
const FE_SRC = path.join(FE_ROOT, 'src/app');
const OUT_DIR = path.join(FE_ROOT, 'i18n-dedup');

const LOCALES = ['vi-VN', 'en-US', 'ja-JP', 'fil-PH'];

const KEY_PATTERNS = [
  /'([a-zA-Z_][\w.-]*)'\s*\|\s*translate/g,
  /"([a-zA-Z_][\w.-]*)"\s*\|\s*translate/g,
  /translate(?:Service)?\.(?:instant|get|stream)\(\s*['"]([a-zA-Z_][\w.-]*)['"]\s*[,)]/g,
  /\b(?:labelKey|titleKey|breadcrumbKey|descKey|descriptionKey|placeholderKey|tooltipKey|messageKey|nameKey|typeModuleKey|headerKey|subtitleKey)\s*:\s*['"]([a-zA-Z_][\w.-]*)['"]/g,
  /\b(?:breadcrumb|title)\s*:\s*['"]([A-Z][A-Z0-9_]+|[a-z][\w]*\.[\w.]+)['"]/g,
];
const SKIP_DIRS = new Set(['node_modules', 'dist', '.angular', 'coverage', '__tests__']);

function flat(obj, prefix = '', out = {}) {
  for (const [k, v] of Object.entries(obj ?? {})) {
    const np = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flat(v, np, out);
    else out[np] = v;
  }
  return out;
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
  // Load locales
  const loc = {};
  for (const l of LOCALES) {
    loc[l] = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, `${l}.json`), 'utf8')));
  }
  const vi = loc['vi-VN'];
  const viPaths = Object.keys(vi);

  // Scan code usage: key -> Set(files); also flag dynamic-key files
  const usedKeys = new Map();
  const dynamicFiles = new Set();
  const DYN = [/['"`][\w.]*\.\s*['"]?\s*\+/, /`[^`]*\$\{[^}]+\}[^`]*`\s*\|\s*translate/];
  for await (const f of walk(FE_SRC)) {
    const c = await fs.readFile(f, 'utf8');
    const rel = path.relative(FE_ROOT, f);
    for (const p of KEY_PATTERNS) {
      p.lastIndex = 0; let m;
      while ((m = p.exec(c)) !== null) {
        const k = m[1];
        if (!k || k.length < 2) continue;
        if (!usedKeys.has(k)) usedKeys.set(k, new Set());
        usedKeys.get(k).add(rel);
      }
    }
    if (DYN.some(re => re.test(c))) dynamicFiles.add(rel);
  }

  // Group vi paths by value
  const byVal = new Map();
  for (const p of viPaths) {
    const v = vi[p];
    if (typeof v !== 'string') continue;
    if (!byVal.has(v)) byVal.set(v, []);
    byVal.get(v).push(p);
  }

  const groups = [];
  for (const [value, paths] of byVal) {
    if (paths.length < 2) continue;
    const perPath = paths.map(p => ({
      path: p,
      vi: vi[p],
      en: loc['en-US'][p] ?? null,
      ja: loc['ja-JP'][p] ?? null,
      fil: loc['fil-PH'][p] ?? null,
      usedCount: usedKeys.has(p) ? usedKeys.get(p).size : 0,
      files: usedKeys.has(p) ? [...usedKeys.get(p)] : [],
    }));
    const enVals = [...new Set(perPath.map(x => x.en).filter(Boolean))];
    const usedPaths = perPath.filter(x => x.usedCount > 0).length;
    groups.push({
      value,
      size: paths.length,
      safe: enVals.length <= 1,        // EN identical => low-risk auto-merge
      enVariants: enVals,
      usedPaths,
      orphanPaths: paths.length - usedPaths,
      perPath,
    });
  }
  groups.sort((a, b) => b.size - a.size);

  const summary = {
    totalViKeys: viPaths.length,
    distinctValues: byVal.size,
    dupGroups: groups.length,
    keysInDupGroups: groups.reduce((s, g) => s + g.size, 0),
    redundantKeys: groups.reduce((s, g) => s + (g.size - 1), 0),
    safeGroups: groups.filter(g => g.safe).length,
    riskyGroups: groups.filter(g => !g.safe).length,
    dynamicKeyFiles: [...dynamicFiles],
    localeKeyCounts: Object.fromEntries(LOCALES.map(l => [l, Object.keys(loc[l]).length])),
  };

  await fs.mkdir(OUT_DIR, { recursive: true });
  await fs.writeFile(path.join(OUT_DIR, 'report.json'), JSON.stringify({ summary, groups }, null, 2));

  // Markdown for humans
  let md = `# i18n dedup report\n\n## Summary\n\n`;
  for (const [k, v] of Object.entries(summary)) {
    md += `- **${k}**: ${Array.isArray(v) ? v.length + ' (' + v.join(', ') + ')' : typeof v === 'object' ? JSON.stringify(v) : v}\n`;
  }
  md += `\n## Risky groups (EN diverges — cần review nghĩa)\n\n`;
  for (const g of groups.filter(g => !g.safe)) {
    md += `### ${g.size}× "${g.value}"  ·  used ${g.usedPaths}/${g.size}\n`;
    md += `EN variants: ${g.enVariants.map(e => `\`${e}\``).join(' | ')}\n\n`;
    for (const p of g.perPath) md += `- \`${p.path}\` → en:\`${p.en}\` (used ${p.usedCount})\n`;
    md += `\n`;
  }
  await fs.writeFile(path.join(OUT_DIR, 'report.md'), md);

  console.log('=== Summary ===');
  console.log(JSON.stringify(summary, null, 2));
  console.log(`\nWrote ${OUT_DIR}/report.json + report.md`);
})().catch(e => { console.error(e); process.exit(1); });
