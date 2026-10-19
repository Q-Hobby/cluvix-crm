import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface AppointmentTypeModel extends Identifiable {
  id?: number;
  name?: string;
  isTypeRevenue?: number;
  status?: number;
  created_at?: null;
  updated_at?: null;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
}
