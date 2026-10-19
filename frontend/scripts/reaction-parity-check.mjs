#!/usr/bin/env node
/**
 * reaction-parity-check — chặn BẢNG EMOJI cảm xúc trôi lệch giữa BE và FE.
 *
 * VÌ SAO CẦN (nợ D7, retro epic omnichat-message-reaction 2026-08-25):
 * Bộ 6 cảm xúc được khai ở HAI nơi, hai ngôn ngữ, hai repo:
 *   - BE  `backend/pkg/define/omni_reaction.go`  → `ReactionEmoji` (nguồn sự thật, giá trị LƯU DB)
 *   - FE  `frontend/src/app/modules/omnichat/omnichat.types.ts` → `OMNI_REACTIONS` (bảng chọn + chip)
 * Không có gì ràng buộc hai bảng này. Nếu trôi lệch:
 *   FE nhận chip mang emoji BE lưu → `toggleChip` tra `palette.find(p => p.emoji === r.emoji)`
 *   → không thấy → **bấm chip không làm gì, IM LẶNG**. Mỗi bên đều tự nhất quán nên không
 *   test/lint nào của riêng repo đó thấy được.
 *
 * ⛔ Script này TUYỆT ĐỐI không được "pass khi không đọc được file". Đó là bẫy đã có sẵn trong
 * `i18n-check.mjs:loadGoKeys` (catch → trả Set rỗng → 0 khoá → không báo lệch gì cả). Ở đây mọi
 * lỗi đọc/parse đều exit 1, và có chốt tự kiểm "phải parse ra > 0 mục ở CẢ HAI bên".
 *
 * Chạy: npm run reaction:parity   (đã gắn vào .husky/pre-commit)
 */
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');
const REPO_ROOT = path.resolve(FE_ROOT, '..');

const BE_FILE = path.join(REPO_ROOT, 'backend/pkg/define/omni_reaction.go');
const FE_FILE = path.join(FE_ROOT, 'src/app/modules/omnichat/omnichat.types.ts');

const fail = (msg) => { console.error(`\n❌ reaction-parity: ${msg}\n`); process.exit(1); };

async function read(file, label) {
  try {
    return await fs.readFile(file, 'utf-8');
  } catch (e) {
    // KHÔNG nuốt: đọc không được nghĩa là lưới KHÔNG chạy, không phải "không có lệch".
    fail(`không đọc được ${label} (${file}): ${e.message}`);
  }
}

/** BE: const ReactionKeyHeart = "heart" → map[tên hằng] = khoá; rồi ReactionEmoji: hằng → emoji. */
function parseBackend(src) {
  const constToKey = new Map();
  for (const m of src.matchAll(/\bReactionKey(\w+)\s*=\s*"([^"]+)"/g)) {
    constToKey.set(`ReactionKey${m[1]}`, m[2]);
  }
  const block = src.match(/var\s+ReactionEmoji\s*=\s*map\[string\]string\{([\s\S]*?)\n\}/);
  if (!block) fail('không tìm thấy `var ReactionEmoji = map[string]string{...}` trong file BE');

  const out = new Map();
  for (const m of block[1].matchAll(/(ReactionKey\w+)\s*:\s*"([^"]+)"/g)) {
    const key = constToKey.get(m[1]);
    if (!key) fail(`ReactionEmoji dùng hằng ${m[1]} nhưng không tìm thấy khai báo khoá của nó`);
    out.set(key, m[2]);
  }
  return out;
}

/** FE: export const OMNI_REACTIONS = [{ key: 'heart', emoji: '❤️' }, ...] as const; */
function parseFrontend(src) {
  const block = src.match(/export\s+const\s+OMNI_REACTIONS\s*=\s*\[([\s\S]*?)\]\s*as\s+const;/);
  if (!block) fail('không tìm thấy `export const OMNI_REACTIONS = [...] as const;` trong file FE');

  const out = new Map();
  for (const m of block[1].matchAll(/\{\s*key:\s*'([^']+)'\s*,\s*emoji:\s*'([^']+)'\s*\}/g)) {
    out.set(m[1], m[2]);
  }
  return out;
}

const be = parseBackend(await read(BE_FILE, 'bảng emoji BE'));
const fe = parseFrontend(await read(FE_FILE, 'bảng emoji FE'));

// Chốt TỰ KIỂM: lưới phải thật sự có gì để soi. Parse ra rỗng mà báo "khớp" là xanh giả.
if (be.size === 0) fail('parse BE ra 0 mục — regex đã lạc hậu so với file, lưới đang mù');
if (fe.size === 0) fail('parse FE ra 0 mục — regex đã lạc hậu so với file, lưới đang mù');

const problems = [];
for (const [key, emoji] of be) {
  if (!fe.has(key)) problems.push(`BE có khoá "${key}" (${emoji}) mà FE KHÔNG có`);
  else if (fe.get(key) !== emoji) {
    problems.push(`khoá "${key}": BE lưu ${emoji} nhưng FE hiển thị ${fe.get(key)}`);
  }
}
for (const key of fe.keys()) {
  if (!be.has(key)) problems.push(`FE có khoá "${key}" mà BE KHÔNG có — gửi lên sẽ bị BE từ chối`);
}

if (problems.length) {
  console.error('\n❌ Bảng emoji cảm xúc BE ↔ FE ĐÃ LỆCH:');
  for (const p of problems) console.error(`   - ${p}`);
  console.error(`\n   BE: ${BE_FILE.replace(REPO_ROOT + '/', '')}  (ReactionEmoji — giá trị LƯU DB)`);
  console.error(`   FE: ${FE_FILE.replace(REPO_ROOT + '/', '')}  (OMNI_REACTIONS)`);
  console.error('\n   Hậu quả nếu để lệch: chip cảm xúc bấm vào KHÔNG làm gì, im lặng.\n');
  process.exit(1);
}

console.log(`✅ reaction-parity: bảng emoji BE ↔ FE khớp (${be.size} cảm xúc)`);
