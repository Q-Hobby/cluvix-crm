
export const GENDER = {
  MALE: 1,
  FEMALE: 2,
  OTHER: 3
} as const;

export const GENDER_LABELS = {
  [GENDER.MALE]: 'Nam',
  [GENDER.FEMALE]: 'Nữ',
  [GENDER.OTHER]: 'Khác'
} as const;

export const GENDER_OPTIONS = [
  { value: GENDER.MALE, label: 'Nam', labelKey: 'common.label.male' },
  { value: GENDER.FEMALE, label: 'Nữ', labelKey: 'common.label.female' },
  { value: GENDER.OTHER, label: 'Khác', labelKey: 'common.label.other' }
];

export const USER_STATUS = {
  DELETED: 1,
  PENDING: 9,
  ACTIVE: 10
} as const;

export const USER_STATUS_LABELS = {
  [USER_STATUS.DELETED]: 'Đã xóa',
  [USER_STATUS.PENDING]: 'Chờ duyệt',
  [USER_STATUS.ACTIVE]: 'Hoạt động'
} as const;

export const USER_STATUS_OPTIONS = [
  { value: USER_STATUS.DELETED, label: 'Đã xóa', labelKey: 'common.deleted' },
  { value: USER_STATUS.PENDING, label: 'Chờ duyệt', labelKey: 'define.user_status.pending' },
  { value: USER_STATUS.ACTIVE, label: 'Hoạt động', labelKey: 'common.status.active' }
];

export const DEPARTMENT_SELECT = {
  technical: 13,
  saleStaff: 12,
  convert: 11,
  directSale: 10,
  boardOfDirectors: 9,
  assistant: 8,
  doctor: 7,
  customerCare: 6,
  accountant: 5,
  marketing: 4,
  telesale: 3,
  receptionist: 2,
  hr: 1,
};

// Helper functions - Optimized for large lists
export const getGenderLabel = (value: number): string => {
  return GENDER_LABELS[value as keyof typeof GENDER_LABELS] || 'Không xác định';
};

// i18n key resolver — returns translation key for runtime language switching.
// Templates: {{ item.genderKey ? (item.genderKey | translate) : item.genderLabel }}
export const getGenderKey = (value: number): string => {
  return GENDER_OPTIONS.find(o => o.value === value)?.labelKey ?? '';
};

export const getUserStatusLabel = (value: number): string => {
  return USER_STATUS_LABELS[value as keyof typeof USER_STATUS_LABELS] || 'Không xác định';
};

// Pre-computed Map for ultra-fast lookup (optional for 1000+ rows)
const GENDER_MAP = new Map<number, string>([
  [GENDER.MALE, 'Nam'],
  [GENDER.FEMALE, 'Nữ'],
  [GENDER.OTHER, 'Khác']
]);

export const getGenderLabelFast = (value: number): string => {
  return GENDER_MAP.get(value) || 'Không xác định';
};
export const ROLE_SELECT = {
  develop: 'develop',
  admin: 'admin',
  sub_admin: 'sub_admin',
  telesales: 'telesales',
  doctor: 'doctor',
  receptionist: 'receptionist',
  convert: 'convert',
  accountant: 'accountant',
  assistant: 'assistant', // Phụ tá
  customer_care: 'customer_care',
  directSale: 'directSale',
  admin_branch: 'admin_branch',
  editor_content: 'editor_content',
  hr: 'hr',
  lead_marketing: 'lead_marketing',
  marketing: 'marketing',
  users: 'users',
};
