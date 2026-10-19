/**
 * Tiện ích DÙNG CHUNG cho các lưới QUÉT MÃ NGUỒN (spec đọc thẳng file `.ts`/`.html` bằng
 * `readFileSync` rồi khớp mẫu, thay vì chạy hành vi).
 *
 * ⛔ Vì sao tồn tại (nợ View-1 mục 6 của retro teamchat): `stripComments` từng bị CHÉP BA BẢN —
 * `teamchat-ref-picker.spec.ts`, `teamchat-ref.spec.ts`, `teamchat-entity-title.spec.ts` — và ba bản
 * đã trôi lệch nhau (chỉ một bản bóc `<!-- -->`). Một lưới quét mã nguồn mà hàm bóc comment của nó
 * khác nhau tuỳ file thì cùng một vi phạm sẽ bị bắt ở spec này và LỌT ở spec kia, im lặng.
 *
 * ⚠ KHÔNG phải file spec (đuôi `.testutil.ts`) nên vitest không thu gom; cũng không nằm trong đồ thị
 * import của `main.ts` nên không vào bundle ứng dụng.
 */

/**
 * Bóc CHÚ THÍCH khỏi mã nguồn TRƯỚC khi khớp mẫu.
 *
 * ⚠ Bắt buộc phải bóc: chính các khối chú thích trong repo nhắc NGUYÊN VĂN những mẫu bị cấm (để giải
 * thích "không được viết thế này nữa") — không bóc thì mọi hit đều là hit GIẢ, và một lưới hay báo
 * giả thì sớm muộn bị nới cho im.
 *
 * ⚠ CẠM BẪY của chính hàm này khi tự kiểm lưới bằng mutation: cấy cờ dạng **comment** sẽ bị bóc ở đây
 * ⇒ test vẫn xanh ⇒ kết luận sai về việc lưới có hoạt động hay không. Mutation phải cấy bằng MÃ THẬT
 * (đổi con số/ký tự trong file sản phẩm).
 *
 * Phạm vi bóc — cố ý HẸP, chỉ những dạng chắc chắn là chú thích thuần:
 *  · khối `/* … *\/` (kể cả JSDoc nhiều dòng)
 *  · khối `<!-- … -->` của template HTML
 *  · DÒNG mở đầu bằng `//` hoặc `*` (dòng giữa của khối JSDoc)
 *
 * Code có chú thích ở CUỐI dòng vẫn giữ nguyên phần code — đúng chủ đích: bóc thêm sẽ cần một parser
 * thật (chuỗi chứa `//`, regex literal chứa `/*`…), và lưới quét mã nguồn không đáng đánh đổi đến đó.
 */
export function stripComments(src: string): string {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .split('\n')
    .filter((l) => !/^\s*(\/\/|\*)/.test(l))
    .join('\n');
}
