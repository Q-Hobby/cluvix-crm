/** @type {import('prettier').Config} */
module.exports = {
  printWidth: 100, // 🌟 Max 100 ký tự 1 dòng (Angular style guide recommend)
  tabWidth: 2, // Tab 2 spaces
  semi: true, // 🌟 Có dấu ";" cuối câu
  singleQuote: true, // 🌟 Ưu tiên dùng dấu nháy đơn ' thay cho "
  trailingComma: 'all', // 🌟 Thêm dấu , cuối object/array (hạn chế diff lỗi khi thêm dòng mới)
  bracketSpacing: true, // 🌟 Có khoảng trắng bên trong {} => { a: 1 }
  arrowParens: 'always', // 🌟 Arrow function luôn có ( ) => (x) => ...
  endOfLine: 'lf', // 🌟 Linebreak dùng LF (Linux/Mac chuẩn git)

  overrides: [
    {
      files: ['*.ts', '*.tsx'],
      options: {
        parser: 'typescript',
      },
    },
    {
      files: ['*.html'],
      options: {
        parser: 'angular',
      },
    },
    {
      files: ['*.scss'],
      options: {
        parser: 'scss',
      },
    },
  ],
};
