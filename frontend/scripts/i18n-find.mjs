#!/usr/bin/env node
/**
 * i18n find — tra cứu ngược theo VALUE trước khi tạo key mới.
 *
 * MỤC ĐÍCH: chống tạo key trùng. TRƯỚC khi add 1 key dịch mới, chạy lệnh này với
 * nội dung tiếng Việt (hoặc Anh) định thêm — nếu đã có key mang value đó thì DÙNG LẠI,
 * KHÔNG tạo key mới.
 *
 * Usage:
 *   node scripts/i18n-find.mjs "Thanh toán"        # khớp chính xác (case-insensitive, bỏ dấu cách thừa)
 *   node scripts/i18n-find.mjs --contains "thanh"   # khớp chứa chuỗi con
 *   node scripts/i18n-find.mjs --lang=en "Payment"  # tra theo en-US thay vì vi-VN
 *
 * Exit code: 0 nếu KHÔNG tìm thấy (an toàn tạo mới) · 3 nếu CÓ key trùng (nên reuse).
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const I18N_DIR = path.resolve(__dirname, '..', 'src/assets/i18n');

const args = process.argv.slice(2);
const contains = args.includes('--contains');
const langArg = (args.find(a => a.startsWith('--lang=')) || '--lang=vi').split('=')[1];
const locale = langArg.startsWith('en') ? 'en-US' : 'vi-VN';
const query = args.filter(a => !a.startsWith('--')).join(' ').trim();

if (!query) { console.error('Usage: i18n-find.mjs [--contains] [--lang=vi|en] "<value>"'); process.exit(1); }

const flat = (o, p = '', out = {}) => {
  for (const [k, v] of Object.entries(o ?? {})) {
    const np = p ? `${p}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flat(v, np, out);
    else out[np] = v;
  }
  return out;
};
const norm = (s) => String(s ?? '').toLowerCase().replace(/\s+/g, ' ').trim();

(async () => {
  const vi = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, 'vi-VN.json'), 'utf8')));
  const en = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, 'en-US.json'), 'utf8')));
  const target = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, `${locale}.json`), 'utf8')));
  const q = norm(query);

  const hits = Object.entries(target).filter(([, v]) =>
    typeof v === 'string' && (contains ? norm(v).includes(q) : norm(v) === q));

  if (!hits.length) {
    console.log(`✅ KHÔNG có key nào mang value "${query}" (${locale}) → an toàn tạo key mới.`);
    console.log(`   Đặt tên ưu tiên: common.<label|action|status|message>.<slug> nếu dùng chung ≥2 module.`);
    process.exit(0);
  }

  console.log(`⚠️  ĐÃ CÓ ${hits.length} key mang value "${query}" (${locale}) → DÙNG LẠI, đừng tạo mới:\n`);
  // group theo nghĩa EN để thấy đồng âm khác nghĩa
  const byEn = new Map();
  for (const [k] of hits) {
    const e = en[k] ?? '(no en)';
    if (!byEn.has(e)) byEn.set(e, []);
    byEn.get(e).push(k);
  }
  for (const [e, keys] of byEn) {
    console.log(`  • EN="${e}"`);
    keys.forEach(k => console.log(`      ${k}`));
  }
  if (byEn.size > 1) console.log(`\n  ⓘ Có ${byEn.size} nghĩa EN khác nhau — chọn key ĐÚNG NGHĨA cho ngữ cảnh của bạn.`);
  process.exit(3);
})().catch(e => { console.error(e); process.exit(1); });
