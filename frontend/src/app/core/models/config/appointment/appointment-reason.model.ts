import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface AppointmentReasonModel extends Identifiable {
  id?: number;
  name?: string;
  SortOrder?: number;
  time_nhaclich?: number;
  status?: number;
  created_at?: null;
  updated_at?: null;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
