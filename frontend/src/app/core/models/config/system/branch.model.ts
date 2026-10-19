import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface BranchModel extends Identifiable {
  id?: number;
  name?: string;
  code?: string;
  address?: string;
  hotline?: string;
  email?: string;
  statusOpenAppointment?: number;
  status?: number;
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;

  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
