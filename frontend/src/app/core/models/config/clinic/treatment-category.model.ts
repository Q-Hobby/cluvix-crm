import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface TreatmentCategoryModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  status?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  SortOrder?: number;
  onOffTreatment?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
