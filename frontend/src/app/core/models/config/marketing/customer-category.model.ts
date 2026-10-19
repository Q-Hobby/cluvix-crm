import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface CustomerCategoryModel extends Identifiable {
  name?: string;
  id?: number;
  status?: number;
  SortOrder?: number;
  time_nhaclich?: number;
  type_nhaclich?: number;
  created_at?: null;
  updated_at?: null;
  created_by?: number;
  updated_by?: number;

  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
