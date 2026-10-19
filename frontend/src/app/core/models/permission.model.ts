import { Identifiable } from '@/core/interface/identifiable';

export interface PermissionModel extends Identifiable {
  id?: number;
  name?: string;
  description?: string;
}
