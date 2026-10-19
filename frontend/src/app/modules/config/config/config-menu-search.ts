import { ConfigMenuGroup, ConfigMenuItem } from '@/modules/config/config/config-menu.registry';
import { normalizeSearchText } from '@/share/utils/text-normalize.util';

/**
 * Tìm kiếm menu /config — hàm THUẦN (không inject gì) để vitest chạy trực tiếp.
 *
 * Nguyên tắc: search chỉ trả về mục user THỰC SỰ mở được — lọc quyền TRƯỚC khi đánh chỉ số,
 * không phải lọc lúc render (search không được lộ ra mục sẽ dẫn tới /auth/accessdenied).
 */

/** Ngữ cảnh quyền — truyền vào thay vì đọc signal, để test không cần dựng auth state. */
export interface PermissionContext {
  isSuperUser: boolean;
  granted: ReadonlySet<string>;
}

export interface ConfigMenuEntry {
  item: ConfigMenuItem;
  group: ConfigMenuGroup;
  /** Nhãn đã dịch theo ngôn ngữ hiện tại — dùng để hiển thị VÀ để highlight theo offset. */
  label: string;
  groupLabel: string;
  /** Nhãn đã bỏ dấu, ĐỘ DÀI BẰNG `label` (xem normalizeAligned) → offset khớp 1-1. */
  labelNorm: string;
  /** Toàn bộ text đánh chỉ số: nhãn + tên nhóm + keywords + link. */
  haystack: string;
}

export interface ConfigMenuHit extends ConfigMenuEntry {
  /** Vị trí khớp trên `label` để highlight; null khi chỉ khớp qua nhóm/keyword/link. */
  match: { start: number; end: number } | null;
}

export function hasPermission(perm: string | string[] | undefined, ctx: PermissionContext): boolean {
  if (!perm) return true;
  if (ctx.isSuperUser) return true;
  const perms = Array.isArray(perm) ? perm : [perm];
  return perms.some((p) => ctx.granted.has(p));
}

export function isItemVisible(group: ConfigMenuGroup, item: ConfigMenuItem, ctx: PermissionContext): boolean {
  if (item.superUserOnly && !ctx.isSuperUser) return false;
  if (!hasPermission(group.permission, ctx)) return false;
  return hasPermission(item.permission, ctx);
}

/**
 * Bỏ dấu THEO TỪNG KÝ TỰ để chuỗi kết quả có cùng số ký tự với chuỗi gốc.
 *
 * `normalizeSearchText` chạy trên cả chuỗi vẫn giữ độ dài với tiếng Việt, nhưng chỉ đúng *tình cờ*
 * (mỗi ký tự tổ hợp rút về đúng 1). Chuẩn hoá từng ký tự thì offset khớp 1-1 là bảo đảm, nhờ đó
 * highlight cắt đúng chỗ trên nhãn CÓ DẤU.
 */
export function normalizeAligned(value: string): string {
  return Array.from(value)
    .map((ch) => {
      const n = normalizeSearchText(ch);
      return n.length === 1 ? n : ch.toLowerCase();
    })
    .join('');
}

export function buildMenuIndex(
  groups: readonly ConfigMenuGroup[],
  ctx: PermissionContext,
  t: (key: string) => string,
): ConfigMenuEntry[] {
  const entries: ConfigMenuEntry[] = [];

  for (const group of groups) {
    const groupLabel = group.titleKey ? t(group.titleKey) : group.title;

    for (const item of group.items) {
      if (!isItemVisible(group, item, ctx)) continue;

      const label = item.labelKey ? t(item.labelKey) : item.label;
      entries.push({
        item,
        group,
        label,
        groupLabel,
        labelNorm: normalizeAligned(label),
        haystack: normalizeSearchText(
          // Nhãn VN gốc luôn nằm trong chỉ số: user quen gõ tiếng Việt kể cả khi UI đang tiếng Anh.
          [label, item.label, groupLabel, group.title, ...(item.keywords ?? []), item.link].join(' '),
        ),
      });
    }
  }

  return entries;
}

export function tokenize(query: string): string[] {
  return normalizeSearchText(query).split(/\s+/).filter(Boolean);
}

/**
 * Lọc + xếp hạng. Mọi token phải khớp (AND) — "nguon khach" không ra mọi mục chứa "khách".
 * Hạng: khớp đầu nhãn > chứa trong nhãn > chỉ khớp nhóm/keyword/link; giữ thứ tự registry khi hoà.
 */
export function searchMenu(entries: readonly ConfigMenuEntry[], query: string): ConfigMenuHit[] {
  const tokens = tokenize(query);
  if (!tokens.length) return [];

  const hits: { hit: ConfigMenuHit; rank: number; order: number }[] = [];

  entries.forEach((entry, order) => {
    if (!tokens.every((tk) => entry.haystack.includes(tk))) return;

    // Highlight theo token DÀI NHẤT khớp được trên nhãn — đoạn tô sát nghĩa nhất với thứ user gõ.
    let match: ConfigMenuHit['match'] = null;
    let rank = 2;
    for (const tk of [...tokens].sort((a, b) => b.length - a.length)) {
      const at = entry.labelNorm.indexOf(tk);
      if (at < 0) continue;
      if (!match) match = { start: at, end: at + tk.length };
      rank = Math.min(rank, at === 0 ? 0 : 1);
    }

    hits.push({ hit: { ...entry, match }, rank, order });
  });

  return hits.sort((a, b) => a.rank - b.rank || a.order - b.order).map((h) => h.hit);
}
