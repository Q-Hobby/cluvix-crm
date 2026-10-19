const { replaceInFileSync } = require('replace-in-file');
const { version } = require('../package.json');

const buildNumber = Date.now();
const swVersion = `${version}.${buildNumber}`;

// replaceInFileSync KHÔNG báo lỗi khi regex trượt — nó "thành công" mà không thay gì cả. Ai đó đổi
// tên hằng / format dòng là version im lặng đứng yên ở giá trị cũ, cache i18n + SW không invalidate
// nữa và không ai biết. Bọc lại để trượt = fail cứng, chặn luôn build.
function replaceOrDie(label, options) {
  const results = replaceInFileSync(options);
  const changed = results.filter((r) => r.hasChanged);
  if (changed.length === 0) {
    console.error(`❌ ${label}: KHÔNG khớp mẫu trong ${options.files} — kiểm tra lại regex/hằng số.`);
    process.exit(1);
  }
  console.log(`✅ ${label} updated to: ${swVersion}`);
}

replaceOrDie('SW_VERSION', {
  files: 'src/custom-ngsw-worker.js',
  from: /const SW_VERSION = ['"].*?['"];/,
  to: `const SW_VERSION = '${swVersion}';`,
});

replaceOrDie('LS_VERSION (i18n cache)', {
  files: 'src/app/core/services/cached-translate-loader.ts',
  from: /const LS_VERSION = ['"].*?['"];/,
  to: `const LS_VERSION = '${swVersion}';`,
});

// Splash trong index.html fetch i18n TRƯỚC khi Angular bootstrap — phải cùng ?v= với loader.
replaceOrDie('I18N_VERSION (index.html splash)', {
  files: 'src/index.html',
  from: /var I18N_VERSION = ['"].*?['"];/,
  to: `var I18N_VERSION = '${swVersion}';`,
});
