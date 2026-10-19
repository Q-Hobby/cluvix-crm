import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface CardTypeModel extends Identifiable {
  name?: string;
  id?: number;
  method?: number;
  status?: number;
  note?: string;
  update_at?: number;
  created_by?: number;
  created_at?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
}
