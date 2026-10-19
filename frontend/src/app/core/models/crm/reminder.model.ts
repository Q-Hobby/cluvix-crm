import { Identifiable } from '@/core/interface/identifiable';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface ReminderModel extends Identifiable {
  id?: number;
  customer_id?: number;
  relatedUser?: any[];
  note?: string;
  status?: number;
  type?: number;
  start_time?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  code?: string;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  customer?: CustomerModel;
}
