import { Identifiable } from '@/core/interface/identifiable';

export interface PrintTemplateModel extends Identifiable {
  id?: number;
  /** 0 = áp dụng cho tất cả chi nhánh */
  branch_id?: number;
  /** Loại phiếu — 'payment' | 'deposit' | 'order' | ... */
  template_type?: string;
  name?: string;
  description?: string;
  /** HTML raw với Handlebars-like syntax: {{customer.fullname}}, {{#each items}}...{{/each}} */
  template_html?: string;
  is_default?: number;
  is_active?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  // Computed label
  templateTypeLabel?: string;
}
