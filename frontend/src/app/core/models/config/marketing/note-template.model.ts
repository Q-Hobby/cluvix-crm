import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface NoteTemplateModel extends Identifiable {
  id?: number;
  type_id?: number;
  name?: string;
  content?: string;
  status?: number;
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;

  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
