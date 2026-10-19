export const UNIT_CATEGORY_SELECT = {
  UNIT_CATEGORY_SERVICE: 1,
  UNIT_CATEGORY_WH: 2,
  UNIT_CATEGORY_MEDICINE: 3,
} as const;

export type UNIT_CATEGORY_INFO = {
  value: number;
  label: string;
};

// CRM standalone: chỉ còn danh mục dịch vụ (kho/thuốc thuộc ERP đã cắt). Hằng _WH/_MEDICINE giữ trong
// UNIT_CATEGORY_SELECT (id danh mục lưu trong DB).
export const UNIT_CATEGORY_OPTIONS = [
  { value: UNIT_CATEGORY_SELECT.UNIT_CATEGORY_SERVICE, label: 'Đơn vị dịch vụ', labelKey: 'define.unit_category.service' },
];

export const UNIT_CATEGORY_MAP = UNIT_CATEGORY_OPTIONS.reduce(
  (acc, item) => {
    acc[item.value] = item;
    return acc;
  },
  {} as Record<number, UNIT_CATEGORY_INFO>,
);
