import { Identifiable } from '@/core/interface/identifiable';

export interface UserOptionModel extends Identifiable {
  id: number;
  name?: string;
  desc?: string;
  options?: number;
  status?: number;
  SortOrder?: number;
}
