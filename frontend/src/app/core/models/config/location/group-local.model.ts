import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface GroupLocalModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  status?: number;
  locationProvinceId?: number[];
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;

  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
