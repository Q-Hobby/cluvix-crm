import { Identifiable } from '@/core/interface/identifiable';

export interface ServiceTreatmentStepModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  status?: number;
  sort_order?: number;
  point?: number;
  commission_type?: number;
  commission_value?: number;
  duration?: number;
  created_at?: number;
  created_by?: number;
}
