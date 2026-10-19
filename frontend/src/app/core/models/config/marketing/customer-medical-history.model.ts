import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface CustomerMedicalHistoryModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  SortOrder?: number;
  status?: number;
  created_at?: any;
  updated_at?: any;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
