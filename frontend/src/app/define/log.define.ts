export const LOG_ACTION_SELECT = {
  LOG_ACTION_CREATE: 1,
  LOG_ACTION_UPDATE: 2,
  LOG_ACTION_DELETE: 3,
} as const;

export const LOG_ACTION_OPTION = [
  { label: 'Tạo mới', labelKey: 'common.action.create', value: LOG_ACTION_SELECT.LOG_ACTION_CREATE, severity: 'success' },
  { label: 'Cập nhật', labelKey: 'common.action.update', value: LOG_ACTION_SELECT.LOG_ACTION_UPDATE, severity: 'info' },
  { label: 'Xoá', labelKey: 'common.action.delete', value: LOG_ACTION_SELECT.LOG_ACTION_DELETE, severity: 'danger' },
];

export const LOG_ACTION_MAP: Record<number, { value: number; label: string; labelKey?: string; severity: string }> = {
  [LOG_ACTION_SELECT.LOG_ACTION_CREATE]: LOG_ACTION_OPTION[0],
  [LOG_ACTION_SELECT.LOG_ACTION_UPDATE]: LOG_ACTION_OPTION[1],
  [LOG_ACTION_SELECT.LOG_ACTION_DELETE]: LOG_ACTION_OPTION[2],
};
