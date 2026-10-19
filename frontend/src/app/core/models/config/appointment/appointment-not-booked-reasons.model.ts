import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface AppointmentNotBookedReasonsModel extends Identifiable {
  id?: number;
  parent_id?: number; // Danh mục cha
  name?: string;
  desc?: string;
  status?: number;
  SortOrder?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;

  parent?: AppointmentNotBookedReasonsModel;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
