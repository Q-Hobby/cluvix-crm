import { Identifiable } from '@/core/interface/identifiable';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Doanh số 1-nhiều ghi theo payment — order_revenue_telesale/doctor_consultant/direct_sale
 * (cùng schema, khác cột FK người phụ trách; BE alias về `revenue_id` chung). Xem
 * `wsrevenue.BaseQuery` (backend/internal/modules/webapp/workspace/wsrevenue/list.go).
 */
export interface OrderRevenueModel extends Identifiable {
  id?: number;
  customer_id?: number;
  order_service_used_id?: number;
  payment_id?: number;
  revenue_id?: number; // người phụ trách — tư vấn viên/bác sĩ tư vấn/direct sale tuỳ nguồn bảng
  branch_id?: number;
  amount?: number;
  confirm_time?: number;

  // flat field từ BE JOIN customer/service (không alias) — xem WorkspaceService.addLabelForRevenue
  fullname?: string;
  phone?: string;
  service_name?: string;

  // computed bởi WorkspaceService.addLabelForRevenue
  customer?: CustomerModel;
  serviceInfo?: { name?: string };
  customerDisplayName?: string;
  amountFormatted?: string;
  confirm_timeFormatted?: string;
  branchLabel?: string;
  revenue?: UserProfileModel;
}
