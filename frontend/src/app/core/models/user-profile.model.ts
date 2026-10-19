import { Identifiable } from '@/core/interface/identifiable';
import { BranchModel } from '@/core/models/config/system/branch.model';

export interface UserProfileModel extends Identifiable {
  id?: number;
  user_id?: number;
  fullname?: string;
  nickname?: string;
  code?: string;
  birthday?: Date;
  about?: string;
  address?: string;
  phone?: string;
  email?: string;
  facebook?: string;
  telegram?: string;
  avatar?: string;
  cover?: string;
  locale?: string;
  gender?: number;
  id_pancake?: any;
  departmentList?: number[];
  branchOffice_id?: number; // Chi nhánh mà nhân viên ngồi làm việc
  branchActive_id?: number; // Chi nhánh mà nhân viên muốn thấy khách hàng
  updated_at?: number;

  status?: number;
  branchId?: number[];

  ext?: number;
  branch?: BranchModel[];
  branchOffice?: BranchModel[];
  branchActive?: BranchModel[];
}
