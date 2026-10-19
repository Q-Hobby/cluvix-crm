import { Identifiable } from '@/core/interface/identifiable';

export interface EthnicityModel extends Identifiable {
  id?: number;
  name?: string;
  codeEthnicity?: string;
  SortOrder?: number;
  status?: number;
  created_at?: number;
}
