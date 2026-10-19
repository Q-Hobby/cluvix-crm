import { Identifiable } from '@/core/interface/identifiable';
import { OrderServiceUsedModel } from '@/core/models/crm/order-service-used.model';

export interface OrderInstallmentModel extends Identifiable {
  id?: number;
  order_service_used_id?: number;
  period_number?: number; // Kỳ thứ mấy (1,2,3...)
  amount?: number; // Số tiền kỳ này
  due_date?: string; // Ngày đến hạn (YYYY-MM-DD)
  paid_date?: string | null; // Ngày thanh toán thực tế
  paid_amount?: number; // Số tiền đã thanh toán
  status?: number; // 1: Chưa thu, 2: Đã thu, 3: Thanh toán 1 phần (khớp BE define.InstallmentStatus*)
  // 1 = kỳ còn mở có due_date < hôm nay (timezone company) — do BE tính. "Quá hạn" KHÔNG phải status.
  is_overdue?: number;
  note?: string | null;
  created_at?: number;

  // Relations
  orderServiceUsed?: OrderServiceUsedModel;
}

export enum InstallmentStatus {
  UNPAID = 1, // Chưa thu
  PAID = 2, // Đã thu
  PARTIAL = 3, // Thanh toán 1 phần
}

export const INSTALLMENT_STATUS_LABELS: Record<InstallmentStatus, string> = {
  [InstallmentStatus.UNPAID]: 'Chưa thu',
  [InstallmentStatus.PAID]: 'Đã thu',
  [InstallmentStatus.PARTIAL]: 'Thanh toán 1 phần',
};
