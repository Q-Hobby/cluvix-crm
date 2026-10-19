export interface CustomerInfoField {
  key: string;
  labelKey: string;
  isDate?: boolean;
  isLink?: boolean;
  condition?: (customer: any) => boolean;
  getValue?: (customer: any) => any;
  getTooltip?: (customer: any) => string;
}

export const CUSTOMER_INFO_FIELDS: CustomerInfoField[] = [
  { key: 'old_code', labelKey: 'crm.customer_info.old_code' },
  { key: 'other_phone', labelKey: 'crm.customer_info.other_phone' },
  { key: 'email', labelKey: 'common.label.email' },
  {
    key: 'gender',
    labelKey: 'common.label.gender',
    getValue: (c) => (c.gender === 1 ? 'common.label.male' : c.gender === 2 ? 'common.label.female' : ''),
  },
  { key: 'address', labelKey: 'common.label.address' },
  { key: 'address_other', labelKey: 'common.label.other_address' },
  { key: 'ward.name', labelKey: 'common.label.ward_commune' },
  { key: 'provinceLabel', labelKey: 'common.label.province_city' },
  { key: 'countryInfo.name', labelKey: 'common.label.country' },
  {
    key: 'relatives',
    labelKey: 'common.label.relatives',
    condition: (c) => c.relatives?.nameRelation,
    getValue: (c) => `${c.relatives.nameRelation} - ${c.relatives.phoneRelation || ''}`.trim(),
  },
  { key: 'birthday', labelKey: 'crm.customer_info.birthday', isDate: true },
  { key: 'data', labelKey: 'common.label.data' },
  { key: 'connect', labelKey: 'common.label.connect' },
  {
    key: 'paymentUserInfo',
    labelKey: 'common.label.consultant',
    getValue: (c) => c.paymentUserInfo?.nickname || c.paymentUserInfo?.fullname,
  },
  { key: 'serviceTypeLabel', labelKey: 'crm.customer_info.interest' },
  { key: 'sourceLabel', labelKey: 'common.label.source_2' },
  { key: 'jobLabel', labelKey: 'common.label.job' },
  { key: 'ethnicityLabel', labelKey: 'common.label.ethnicity' },
  { key: 'customerTypeLabel', labelKey: 'crm.customer_info.customer_type' },
  { key: 'customerCategory', labelKey: 'crm.customer_info.category', getValue: (c) => c.customerCategoryLabel },
  { key: 'loai_message', labelKey: 'crm.customer_info.ad' },
  { key: 'chat_message', labelKey: 'crm.customer_info.chat', isLink: true },
  {
    key: 'facebook',
    labelKey: 'common.label.facebook',
    isLink: true,
    getTooltip: (c) => c.facebook,
  },
  {
    key: 'instagram',
    labelKey: 'crm.customer_info.instagram',
    isLink: true,
    getTooltip: (c) => c.instagram,
  },
  {
    key: 'viber',
    labelKey: 'crm.customer_info.viber',
    isLink: true,
    getTooltip: (c) => c.viber,
  },
  {
    key: 'zalo',
    labelKey: 'common.label.zalo',
    isLink: true,
    getTooltip: (c) => c.zalo,
  },
  { key: 'code_affiliate', labelKey: 'common.label.referral_code' },
  { key: 'affiliate_fullname', labelKey: 'common.label.referrer' },
  { key: 'createdByInfo.fullname', labelKey: 'common.label.user_created' },
  { key: 'created_dayFormatted', labelKey: 'common.label.collection_date' },
  { key: 'created_atFormatted', labelKey: 'common.label.created_at' },
  { key: 'updatedByInfo.fullname', labelKey: 'common.label.user_updated' },
  { key: 'updated_atFormatted', labelKey: 'common.label.updated_at' },
];
