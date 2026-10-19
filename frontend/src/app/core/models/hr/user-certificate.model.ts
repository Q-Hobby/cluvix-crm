import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Chứng chỉ hành nghề của nhân viên (bảng `user_certificate`) — CCHN khám chữa bệnh,
 * chứng chỉ chuyên khoa... phục vụ compliance Sở Y tế.
 */
export interface UserCertificateModel extends Identifiable {
  id?: number;
  user_id?: number;
  /** 1=CCHN khám chữa bệnh, 2=Chứng chỉ chuyên khoa, 3=Khác (tên tự do qua type_name). */
  type?: number;
  /** Tên chứng chỉ tự do khi type=Khác. */
  type_name?: string | null;
  /** Số chứng chỉ. */
  cert_no?: string | null;
  /** Phạm vi hành nghề. */
  scope_of_practice?: string | null;
  /** Nơi cấp. */
  issued_by?: string | null;
  /** "YYYY-MM-DD". */
  issued_date?: string | null;
  /** "YYYY-MM-DD" — null = vô thời hạn. */
  expiry_date?: string | null;
  /** Link file scan chứng chỉ. */
  file_url?: string | null;
  note?: string | null;
  status?: number;
  created_by?: number | null;
  updated_by?: number | null;
  created_at?: number;
  updated_at?: number;
  userProfile?: UserProfileModel;
}
