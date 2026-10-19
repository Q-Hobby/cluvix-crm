export const REMINDER_TYPE_SELECT = {
  REMINDER_TYPE_CALL: 1,
  REMINDER_TYPE_CHAT_FB: 2,
  REMINDER_TYPE_CONG_VIEC: 3,
  REMINDER_TYPE_GAP_MAT: 4,
} as const;

export const REMINDER_TYPE_OPTIONS = [
  { label: 'Gọi điện', labelKey: 'common.label.phone_call', value: REMINDER_TYPE_SELECT.REMINDER_TYPE_CALL },
  { label: 'Chat FB', labelKey: 'define.reminder.type.chat_fb', value: REMINDER_TYPE_SELECT.REMINDER_TYPE_CHAT_FB },
  { label: 'Công việc', labelKey: 'define.reminder.type.task', value: REMINDER_TYPE_SELECT.REMINDER_TYPE_CONG_VIEC },
  { label: 'Gặp mặt', labelKey: 'define.reminder.type.meeting', value: REMINDER_TYPE_SELECT.REMINDER_TYPE_GAP_MAT },
];

export const REMINDER_TYPE_MAP = REMINDER_TYPE_OPTIONS.reduce(
  (acc, item) => {
    acc[item.value] = item;
    return acc;
  },
  {} as Record<number, { value: number; label: string; labelKey: string }>,
);

export const REMINDER_STATUS_SELECT = {
  STATUS_PLANED: 1,
  STATUS_DONE: 2,
} as const;

export const REMINDER_STATUS_OPTIONS = [
  { label: 'Lên kế hoạch', labelKey: 'define.reminder.planed', value: REMINDER_STATUS_SELECT.STATUS_PLANED, severity: 'warn' },
  { label: 'Đã thực hiện', labelKey: 'define.reminder.done', value: REMINDER_STATUS_SELECT.STATUS_DONE, severity: 'success' },
];

export const REMINDER_STATUS_MAP = REMINDER_STATUS_OPTIONS.reduce(
  (acc, item) => {
    acc[item.value] = item;
    return acc;
  },
  {} as Record<number, { value: number; label: string; labelKey: string; severity: string }>,
);
