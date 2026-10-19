import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface PaymentMethodModel extends Identifiable {
  id?: number;
  name?: string;
  status?: number;
  SortOrder?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
