import { Identifiable } from '@/core/interface/identifiable';

export interface ConfigCodeMonthly extends Identifiable {
  id?: number;
  config_code_id?: number;
  year?: number;
  month?: number;
  last_number?: number;
}
