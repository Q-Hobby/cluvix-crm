import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface AppointmentIsAcceptedModel extends Identifiable {
  id?: number;
  name?: string;
  severity?: string;
  desc?: string | null;
  status?: number;
  isAccepted?: number;
  update_at?: number;
  created_by?: number;
  created_at?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
}
