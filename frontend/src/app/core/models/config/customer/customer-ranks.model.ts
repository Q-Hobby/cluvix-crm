import { Identifiable } from '@/core/interface/identifiable';

export interface CustomerRanksModel extends Identifiable {
  id?: number;
  name?: string;
  min_points?: number;
  max_points?: number;
  discount_percent?: number;
  priority_level?: number;
  color_code?: string;
  description?: string;
  status?: number;
  created_at?: number;
}
