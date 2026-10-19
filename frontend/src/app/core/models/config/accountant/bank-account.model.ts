import { Identifiable } from '@/core/interface/identifiable';

export interface BankAccountModel extends Identifiable {
  id?: number;
  bank_code?: string;      // Mã ngân hàng theo chuẩn VietQR/Napas (VCB, TCB, MB...)
  account_no?: string;     // Số tài khoản ngân hàng
  account_name?: string;   // Tên chủ tài khoản
  description?: string;    // Mô tả tài khoản
  sort_order?: number;     // Thứ tự hiển thị
  status?: number;         // Trạng thái (từ BaseModel)
  is_default?: number;     // Tài khoản mặc định (0: không, 1: có)
  created_by?: number;     // ID người tạo (từ BaseModel)
  updated_by?: number;     // ID người cập nhật (từ BaseModel)
  created_at?: number;     // Thời gian tạo
  updated_at?: number;     // Thời gian cập nhật
}
