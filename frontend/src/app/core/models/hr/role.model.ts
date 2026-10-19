import { Identifiable } from '@/core/interface/identifiable';

export interface RoleModel extends Identifiable {
  id?: number; // Khai báo cho có
  name?: string;
  status?: null;
  description?: string;
  created_at?: any;
  updated_at?: any;
  SortOrder?: number;
  label?: string;
  value?: string;
}
