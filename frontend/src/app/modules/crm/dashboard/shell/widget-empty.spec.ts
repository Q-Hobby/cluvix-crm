import { describe, expect, it } from 'vitest';

import { isEmptyChartRows, isEmptyClv, isEmptyList, isEmptyObject, isEmptyRows, isEmptyServiceStats } from './widget-empty';

describe('điều kiện rỗng theo giá trị BE trả khi không có dữ liệu', () => {
  it('danh sách: [] rỗng, có phần tử không rỗng', () => {
    expect(isEmptyList([])).toBe(true);
    expect(isEmptyList(null)).toBe(true);
    expect(isEmptyList([{ name: 'a', value: 1 }])).toBe(false);
  });
  it('marketing-view: {} rỗng (chưa cấu hình nguồn online)', () => {
    expect(isEmptyObject({})).toBe(true);
    expect(isEmptyObject({ budget: 0 })).toBe(false);
  });
  it('CLV: {"totalComputed":0} rỗng', () => {
    expect(isEmptyClv({ totalComputed: 0 })).toBe(true);
    expect(isEmptyClv({ totalComputed: 3 })).toBe(false);
  });
  it('biểu đồ: không có data rỗng', () => {
    // Payload thật có thêm `categories` — biến trung gian để TS không bắt lỗi excess-property (kiểu nhận chỉ khai `data`).
    const empty = { categories: [], data: [] };
    expect(isEmptyChartRows(empty)).toBe(true);
    expect(isEmptyChartRows({ data: [[1, 'a']] })).toBe(false);
  });
  it('name/value: [] hoặc TOÀN 0 ⇒ rỗng (phễu toàn "0" — QA P1); có 1 giá trị ≠ 0 ⇒ không rỗng', () => {
    expect(isEmptyRows([])).toBe(true);
    expect(isEmptyRows([{ name: 'a', value: 0 }, { name: 'b', value: 0 }])).toBe(true);
    expect(isEmptyRows([{ name: 'a', value: 0 }, { name: 'b', value: 3 }])).toBe(false);
    expect(isEmptyRows([{ name: 'a', value: '0' }])).toBe(true);
  });
  it('thống kê dịch vụ: toàn quantity 0 + revenue 0 ⇒ rỗng', () => {
    expect(isEmptyServiceStats([{ name: 'x', quantity: 0, revenue: '0' }])).toBe(true);
    expect(isEmptyServiceStats([{ name: 'x', quantity: 0, revenue: '100' }])).toBe(false);
  });
});
