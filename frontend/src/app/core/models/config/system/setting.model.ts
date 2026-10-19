import { Identifiable } from '@/core/interface/identifiable';

export interface SettingModel extends Identifiable {
  id?: number;
  name?: string;
  key?: string;
  value?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
}
