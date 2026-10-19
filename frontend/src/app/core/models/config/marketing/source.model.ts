import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface SourceModel extends Identifiable {
  id?: number;
  name?: string;
  status?: number;
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;

  userUpdated?: UserProfileModel;
  userCreated?: UserProfileModel;
}
