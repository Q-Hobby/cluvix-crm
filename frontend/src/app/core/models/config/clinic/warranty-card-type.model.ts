import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface WarrantyCardTypeModel extends Identifiable {
  name?: string;
  desc?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  id?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
  images?: string;
}
