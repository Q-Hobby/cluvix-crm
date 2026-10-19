import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface HrUserContractModel extends Identifiable {
  id?: number;
  version?: number;
  user_id?: number;
  branch_id?: number | null;
  contract_code?: string | null;
  contract_type?: number;
  position_id?: number | null;
  department_id?: number | null;
  manager_id?: number | null;
  sign_date?: string | null;
  effective_date?: string;
  end_date?: string | null;
  probation_days?: number;
  salary_type?: number;
  base_salary?: number | string;
  probation_salary?: number | string | null;
  standard_workdays?: number | string;
  insurance_salary?: number | string | null;
  insurance_status?: number;
  dependents_count?: number;
  file_url?: string | null;
  signed_by?: number | null;
  note?: string;
  status?: number;
  created_by?: number | null;
  updated_by?: number | null;
  created_at?: number;
  updated_at?: number;
  userProfile?: UserProfileModel;
}
