import { describe, expect, it } from 'vitest';

import { sanitizeUserHtml } from './sanitize-user-html';

/**
 * Lưới cho allowlist của `SafeHtmlPipe`. Pipe này là lớp chặn XSS DUY NHẤT cho mọi nội dung do người
 * dùng nhập được render bằng `[innerHTML]` (ghi chú lịch hẹn, timeline, tin nhắn omnichat, mẫu in).
 *
 * Test đi HAI CHIỀU có chủ đích:
 * - chiều tấn công: payload phải bị lọc;
 * - chiều nội dung thật: định dạng hợp lệ KHÔNG được nuốt — allowlist siết quá tay thì dữ liệu cũ mất
 *   hiển thị mà không có lỗi nào báo, đúng loại hỏng khó phát hiện nhất.
 */
describe('sanitizeUserHtml — chặn payload nguy hiểm', () => {
  it.each([
    ['script', '<script>alert(1)</script>ghi chú'],
    ['img onerror', '<img src=x onerror="alert(1)">'],
    ['href javascript:', '<a href="javascript:alert(1)">click</a>'],
    ['iframe', '<iframe src="https://evil.tld"></iframe>'],
    ['svg onload', '<svg/onload=alert(1)>'],
    ['form + input', '<form action="/x"><input name="pwd"></form>'],
  ])('lọc %s', (_name, payload) => {
    const out = sanitizeUserHtml(payload);
    expect(out).not.toMatch(/<script|onerror|onload|javascript:|<iframe|<svg|<input|<form/i);
  });

  // DOMPurify KHÔNG tự lọc giá trị CSS — 3 ca dưới đây chỉ pass nhờ hook `uponSanitizeAttribute`
  // trong pipe. Xoá hook đi là cả 3 đỏ ngay.
  it('lọc CSS nạp tài nguyên / thực thi', () => {
    expect(sanitizeUserHtml('<div style="background:url(javascript:alert(1))">x</div>')).not.toContain('javascript:');
    expect(sanitizeUserHtml('<p style="width: expression(alert(1))">x</p>')).not.toContain('expression(');
  });

  it('lọc CSS dựng lớp phủ (clickjacking)', () => {
    const out = sanitizeUserHtml('<div style="position:fixed;top:0;left:0;z-index:9999">x</div>');
    expect(out).not.toMatch(/position\s*:|z-index/i);
  });
});

describe('sanitizeUserHtml — giữ nguyên định dạng hợp lệ', () => {
  it('giữ định dạng chữ, danh sách, tiêu đề, đường kẻ', () => {
    const out = sanitizeUserHtml(
      '<h2>Tiêu đề</h2><p><strong>đậm</strong> <em>nghiêng</em> <u>gạch chân</u> <s>gạch ngang</s></p>' +
        '<ol><li>một</li></ol><blockquote>trích</blockquote><pre><code>ma()</code></pre><hr>',
    );
    for (const tag of ['h2', 'strong', 'em', 'u', 's', 'ol', 'li', 'blockquote', 'pre', 'code', 'hr']) {
      expect(out, `mất tag <${tag}>`).toContain(`<${tag}`);
    }
  });

  it('giữ màu chữ, canh lề và thụt đầu dòng của Quill', () => {
    const out = sanitizeUserHtml(
      '<p style="text-align: center; padding-left: 3em;"><span style="color: rgb(230, 0, 0);" class="ql-size-large">màu</span></p>',
    );
    expect(out).toContain('text-align');
    expect(out).toContain('padding-left');
    expect(out).toContain('color');
    expect(out).toContain('ql-size-large');
  });

  it('giữ ảnh lazy-load (LazyImgPipe chèn loading/decoding trước khi qua pipe này)', () => {
    const out = sanitizeUserHtml('<img loading="lazy" decoding="async" src="/x.png" alt="a" width="120">');
    expect(out).toContain('loading=');
    expect(out).toContain('decoding=');
  });

  it('giữ bảng có ô gộp và checklist data-checked', () => {
    expect(sanitizeUserHtml('<table><tbody><tr><td colspan="2">a</td><th rowspan="2">b</th></tr></tbody></table>'))
      .toMatch(/colspan="2"[\s\S]*rowspan="2"/);
    expect(sanitizeUserHtml('<ul><li data-checked="true">xong</li></ul>')).toContain('data-checked');
  });

  it('giữ link ngoài kèm target/rel', () => {
    const out = sanitizeUserHtml('<a href="https://example.com" target="_blank" rel="noopener">link</a>');
    expect(out).toContain('href="https://example.com"');
    expect(out).toContain('rel="noopener"');
  });
});
