/** Biến đổi dữ liệu biểu đồ Dashboard CRM — hàm THUẦN (story-08), tách khỏi component để test. */
export interface NameValue {
  name: string;
  value: number;
}

// Tên GIỮ ĐẦY ĐỦ trong dữ liệu — rút gọn chỉ ở nhãn/legend qua input `*LabelMaxLength` của directive biểu đồ
// (`share/directive/echart/chart/chart-label.util.ts`), để tooltip còn hiện tên đầy đủ (/check-ui 2026-10-09).

/**
 * Nguồn khách (BE `overview-chart-source`: `{name, value, group}[]`) ⇒ biểu đồ tròn lồng:
 * vòng ngoài top 10 nguồn + "Nguồn khác"; vòng trong top 5 nhóm nguồn + "Nhóm khác". Nhãn "khác" truyền vào đã dịch.
 */
export function sourceNestedPie(
  rows: readonly { name: string; value: number; group: string }[],
  labels: { otherSources: string; otherGroups: string },
): { inner: NameValue[]; outer: NameValue[] } {
  const outerAll = rows.map((r) => ({ name: r.name, value: r.value })).sort((a, b) => b.value - a.value);
  const outer = outerAll.slice(0, 10);
  const outerOthers = outerAll.slice(10).reduce((s, r) => s + r.value, 0);
  if (outerOthers > 0) outer.push({ name: labels.otherSources, value: outerOthers });

  const groups = new Map<string, number>();
  for (const r of rows) groups.set(r.group, (groups.get(r.group) ?? 0) + r.value);
  const sorted = Array.from(groups, ([name, value]) => ({ name, value })).sort((a, b) => b.value - a.value);
  const inner = sorted.slice(0, 5).map((g) => ({ name: g.name, value: g.value }));
  const innerOthers = sorted.slice(5).reduce((s, g) => s + g.value, 0);
  if (innerOthers > 0) inner.push({ name: labels.otherGroups, value: innerOthers });
  return { inner, outer };
}

/** Thống kê dịch vụ (BE `{name, quantity, revenue}[]`) ⇒ trục kép. */
export function serviceStatsSeries(rows: readonly { name: string; quantity: number; revenue: number | string }[]): {
  categories: string[];
  quantities: number[];
  revenues: number[];
} {
  return {
    categories: rows.map((r) => r.name),
    quantities: rows.map((r) => r.quantity),
    revenues: rows.map((r) => Number(r.revenue)),
  };
}
