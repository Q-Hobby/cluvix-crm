import { Identifiable } from '@/core/interface/identifiable';

export interface CustomerRankServicesModel extends Identifiable {
  id?: number;
  rank_id?: number;
  service_ids?: number[];
  discount_percent?: number;
  status?: number;
  created_at?: number;
}
