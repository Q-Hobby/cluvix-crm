import { Identifiable } from '@/core/interface/identifiable';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { ConfigCodeMonthly } from '@/core/models/config/config-code-monthly.model';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface ConfigCodeModel extends Identifiable {
  id?: number;
  branch_id?: number;
  name?: string;
  parent_table?: string;
  prefix_code?: string;
  infix_code?: string;
  include_branch_code?: boolean;
  monthly_reset?: boolean;
  serial_length?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  branch?: BranchModel;
  configCodeMonthly?: ConfigCodeMonthly;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}

/** GET config/config-code/customer-code-option — kiểu mã khách BE đang áp dụng. */
export interface CustomerCodeOptionInfo {
  option: number;
  default_option: number;
}
