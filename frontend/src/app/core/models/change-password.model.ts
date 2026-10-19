import { Identifiable } from '@/core/interface/identifiable';

export interface ChangePasswordModel extends Identifiable {
  id?: number;
  oldpassword?: string;
  newpassword?: string;
  newpasswordrepeat?: string;
}
