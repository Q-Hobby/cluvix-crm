// eslint.config.js
import tsParser from '@typescript-eslint/parser';
import angularPlugin from '@angular-eslint/eslint-plugin';
import angularTplPlugin from '@angular-eslint/eslint-plugin-template';
import angularTplParser from '@angular-eslint/template-parser';

export default [
  { ignores: ['**/dist/**', '**/node_modules/**', 'coverage/**'] },

  // ===== .ts =====
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        // Chọn 1 trong 2:
        // project: ['./tsconfig.json'],    // type-aware chặt chẽ
        projectService: true, // gọn/nhanh hơn (TS-ESLint v8+)
        tsconfigRootDir: import.meta.dirname,
        sourceType: 'module',
        ecmaVersion: 'latest',
      },
    },
    plugins: {
      '@angular-eslint': angularPlugin,
      '@angular-eslint/template': angularTplPlugin, // để dùng processor
    },
    // ✅ ĐÚNG processor cho inline template
    processor: '@angular-eslint/template/extract-inline-html',
    rules: {
      ...angularPlugin.configs.recommended.rules,
      '@angular-eslint/component-class-suffix': 'off',
      // Tắt auto-fix cho các rules có thể gây lỗi cú pháp
      '@typescript-eslint/member-ordering': 'off',
    },
  },

  // ===== *.spec.ts =====
  // `projectService: true` chỉ gắn file vào `tsconfig.json`, mà file đó lại `exclude: ["**/*.spec.ts"]`
  // ⇒ mọi spec báo "was not found by the project service" và KHÔNG được lint dòng nào (lỗi parse
  // nuốt luôn cả file). Trỏ riêng nhóm spec sang `tsconfig.spec.json` — nơi thực sự include chúng.
  // `projectService: false` là bắt buộc: flat config gộp `parserOptions` theo từng khoá, để cả hai
  // thì parser nhận đồng thời project service lẫn project → xung đột.
  {
    files: ['**/*.spec.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        projectService: false,
        project: ['./tsconfig.spec.json'],
        tsconfigRootDir: import.meta.dirname,
        sourceType: 'module',
        ecmaVersion: 'latest',
      },
    },
  },

  // ===== .html =====
  {
    files: ['**/*.html'],
    languageOptions: { parser: angularTplParser },
    plugins: { '@angular-eslint/template': angularTplPlugin },
    rules: {
      ...angularTplPlugin.configs.recommended.rules,
      // `x != null` / `x == null` là idiom CỐ Ý: bắt cả null lẫn undefined trong một phép so sánh.
      // Ép sang `!==` sẽ ĐỔI HÀNH VI (undefined lọt lưới), còn viết `x !== null && x !== undefined`
      // thì với biểu thức signal (`svc.activeChannelId()`) là gọi hàm hai lần mỗi chu kỳ CD.
      // Bật đúng option của chính rule; mọi `==`/`!=` KHÁC (so số, so chuỗi) vẫn bị bắt như cũ.
      '@angular-eslint/template/eqeqeq': ['error', { allowNullOrUndefined: true }],
    },
  },
];
