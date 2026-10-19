import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface NoteAnalysisModel extends Identifiable {
  name?: string;
  id?: number;
  status?: number;
  desc?: string;
  created_at?: null;
  updated_at?: null;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  SortOrder?: number;
}
