// Hậu kỳ build: thay `const S3_ORIGIN = '';` trong service worker OUTPUT bằng $SW_S3_ORIGIN.
// KHÔNG đụng file nguồn. Không có biến ⇒ giữ rỗng + cảnh báo.
const fs = require('fs');
const path = require('path');

const origin = (process.env.SW_S3_ORIGIN || '').trim().replace(/\/+$/, '');
if (!origin) {
  console.warn('⚠ SW_S3_ORIGIN chưa đặt — service worker giữ S3_ORIGIN rỗng (không cache asset S3).');
  process.exit(0);
}
if (!/^https:\/\/[^\s'"\\]+$/.test(origin)) {
  console.error(`❌ SW_S3_ORIGIN không hợp lệ: ${origin}`);
  process.exit(1);
}

const angular = JSON.parse(fs.readFileSync(path.join(__dirname, '../angular.json'), 'utf8'));
const project = Object.values(angular.projects)[0];
const out = project.architect.build.options.outputPath;
const base = typeof out === 'string' ? out : out.base;
const browser = typeof out === 'string' ? '' : (out.browser ?? 'browser');
const file = path.join(__dirname, '..', base, browser, 'custom-ngsw-worker.js');

if (!fs.existsSync(file)) {
  console.error(`❌ Không thấy ${file} — chạy sau ng build.`);
  process.exit(1);
}
const src = fs.readFileSync(file, 'utf8');
const needle = "const S3_ORIGIN = '';";
const count = src.split(needle).length - 1;
if (count !== 1) {
  console.error(`❌ Mong đợi đúng 1 chỗ "${needle}" trong ${file}, thấy ${count}.`);
  process.exit(1);
}
fs.writeFileSync(file, src.replace(needle, () => `const S3_ORIGIN = '${origin}';`));
console.log(`✅ SW S3_ORIGIN = ${origin} (${file})`);
