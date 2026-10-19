#!/usr/bin/env node
/**
 * i18n-audit — tìm key i18n ĐƯỢC DÙNG trong code nhưng THIẾU ở một hoặc nhiều locale.
 *
 * Vì sao cần: `i18n.T`/`TranslatePipe` khi không tìm thấy key sẽ trả về CHÍNH CÁI KEY, nên màn hình
 * hiện chuỗi thô kiểu `hr.dashboard.kpi_new` mà build vẫn xanh, test vẫn qua, không log gì. Key chỉ
 * có ở vi-VN thì ngôn ngữ còn lại lặng lẽ hỏng — người Việt review sẽ không bao giờ thấy.
 *
 * Quét 3 dạng dùng key, KHÔNG chỉ `| translate`:
 *   1. `'a.b.c' | translate`
 *   2. `label="a.b.c"`  — component tự dịch bên trong (app-slide-over-section/row…)
 *   3. `labelKey: 'a.b.c'` và chuỗi `'common.*' | 'crm.*' | 'define.*'` trong .ts (hằng số define)
 *
 * Giới hạn đã biết (cố ý bỏ qua, không phải bug):
 *   - key ghép động (`'define.emr.record_type.' + x`) — quét tĩnh không dựng được đuôi;
 *   - file `*.spec.ts` — chuỗi trong test là dữ liệu giả, không phải key thật.
 *
 * Dùng:  npm run i18n:audit            (quét src/app)
 *        node scripts/i18n-audit.mjs <path...>
 * Exit code 1 khi có key thiếu → gắn được vào CI/pre-commit.
 */
import fs from 'fs';
import path from 'path';

const LOCALES = ['vi-VN', 'en-US'];
const ROOTS = process.argv.slice(2).length ? process.argv.slice(2) : ['src/app'];

const flat = {};
for (const loc of LOCALES) {
  const data = JSON.parse(fs.readFileSync(`src/assets/i18n/${loc}.json`, 'utf8'));
  const map = {};
  (function walk(obj, prefix) {
    for (const k of Object.keys(obj)) {
      const v = obj[k];
      const key = prefix ? `${prefix}.${k}` : k;
      if (v && typeof v === 'object' && !Array.isArray(v)) walk(v, key);
      else map[key] = v;
    }
  })(data, '');
  flat[loc] = map;
}

const files = ROOTS.flatMap((p) =>
  fs.statSync(p).isDirectory()
    ? fs
        .readdirSync(p, { recursive: true })
        .map((f) => path.join(p, f))
        .filter((f) => /\.(html|ts)$/.test(f) && !f.endsWith('.spec.ts'))
    : [p],
);

const used = new Map();
const add = (key, file) => {
  if (key.endsWith('.')) return; // key ghép động
  if (!used.has(key)) used.set(key, new Set());
  used.get(key).add(path.relative('src/app', file));
};

for (const f of files) {
  const src = fs.readFileSync(f, 'utf8');
  for (const m of src.matchAll(/'([a-z][a-zA-Z0-9_]*(?:\.[a-zA-Z0-9_]+)+)'\s*\|\s*translate/g)) add(m[1], f);
  for (const m of src.matchAll(/\blabel="([a-z][a-zA-Z0-9_]*(?:\.[a-zA-Z0-9_]+)+)"/g)) add(m[1], f);
  for (const m of src.matchAll(/labelKey:\s*'([^']+)'/g)) add(m[1], f);
  for (const m of src.matchAll(/'((?:common|crm|define)\.[a-zA-Z0-9_.]+)'/g)) add(m[1], f);
}

// Key BE gửi xuống động (scripts/i18n-dynamic-keys.json): không có literal ở FE nên quét tĩnh không thấy.
// Ép chúng vào `used` để vẫn được kiểm là có đủ ở mọi locale (prefix thì chỉ áp cho key ĐÃ có trong JSON).
const dyn = JSON.parse(fs.readFileSync('scripts/i18n-dynamic-keys.json', 'utf8'));
for (const k of dyn.keys ?? []) add(k, 'scripts/i18n-dynamic-keys.json');
console.log(`i18n-audit: allowlist động ${(dyn.keys ?? []).length} key + ${(dyn.prefixes ?? []).length} prefix.`);

const problems = [];
for (const [key, where] of [...used].sort()) {
  const missing = LOCALES.filter((l) => flat[l][key] === undefined);
  if (missing.length) problems.push({ key, missing, where: [...where] });
}

console.log(`i18n-audit: ${files.length} file, ${used.size} key.`);
if (!problems.length) {
  console.log(`✓ Mọi key đều có đủ ở cả ${LOCALES.length} locale.`);
  process.exit(0);
}
console.log(`\n${problems.length} KEY THIẾU:\n`);
for (const p of problems) {
  console.log(`  ✗ ${p.key}`);
  console.log(`      thiếu ở: ${p.missing.join(', ')}`);
  console.log(`      dùng trong: ${p.where.join(', ')}`);
}
process.exit(1);
