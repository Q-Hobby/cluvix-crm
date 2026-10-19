import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface ServiceComboTypeModel extends Identifiable {
  id?: number;
  name?: string;
  status?: number;
  description?: string;
  created_at?: null;
  updated_at?: null;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
}
