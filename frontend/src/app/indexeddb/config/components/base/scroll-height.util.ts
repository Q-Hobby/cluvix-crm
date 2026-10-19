export function calculateScrollHeight(
  itemCount: number,
  itemHeight: number = 44,
  maxHeight: number = 200,
): string {
  const calculatedHeight = Math.min(itemCount * itemHeight, maxHeight);
  return `${Math.max(calculatedHeight, itemHeight * 2)}px`; // Tối thiểu 2 items
}
