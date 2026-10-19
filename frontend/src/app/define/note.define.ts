export const NOTE_TEMPLATE_TYPE_SELECT = {
  NOTE_TEMPLATE_TYPE_APPOINTMENT: 1,
  NOTE_TEMPLATE_TYPE_CHECK_UP: 2,
} as const;

export const NOTE_TEMPLATE_TYPE_OPTIONS = [
  { value: NOTE_TEMPLATE_TYPE_SELECT.NOTE_TEMPLATE_TYPE_APPOINTMENT, label: 'Lịch hẹn', labelKey: 'common.label.appointment' },
  { value: NOTE_TEMPLATE_TYPE_SELECT.NOTE_TEMPLATE_TYPE_CHECK_UP, label: 'Tư vấn', labelKey: 'define.note.template_type.check_up' },
];
