import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface GroupSourceModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  SortOrder?: number;
  online?: boolean;
  sourceId?: number[];
  status?: number;
  updated_at?: any;
  created_at?: any;
  created_by?: any;
  updated_by?: any;

  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
