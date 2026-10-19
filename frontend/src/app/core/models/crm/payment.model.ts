import { Identifiable } from '@/core/interface/identifiable';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';
import { PaymentMethodModel } from '@/core/models/config/accountant/payment-method.model';
import { SourceModel } from '@/core/models/config/marketing/source.model';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { OrderServiceUsedModel } from '@/core/models/crm/order-service-used.model';
import { OrderPlanModel } from '@/core/models/order-plan.model';
import { UserProfileModel } from '@/core/models/user-profile.model';
import { ServiceModel } from '@/core/models/config/clinic/service.model';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';

export interface PaymentModel extends Identifiable {
  id?: number;
  code?: string;
  customer_id?: number;
  parent_id?: number;
  serviceUsed_id?: number; // Thanh toán cho dịch vụ nào
  branch_id?: number;
  method?: number;
  card_type_id?: number;
  bank_code?: string;
  payment_time?: number | null; // Ngày thu tiền
  payment_of_date?: number | null; // Ngày thanh toán
  amount?: number;
  amount_type?: number;
  deposit_after?: number;
  deposit_id?: number;
  deposit_type?: number;
  payment_for?: number;
  note?: string;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;

  // `labelKey` là khoá i18n của TYPE_DEPOSIT_OPTION; `label` là chuỗi tiếng Việt cứng trong define
  // (các list cũ đang đọc). Nơi nào dịch được thì ưu tiên labelKey.
  typeDepositInfo?: { value: number; label: string; labelKey?: string };
  amountFormatted?: string;
  branchLabel?: string;
  provinceLabel?: string;
  paymentMethodLabel?: string;
  paymentTypeLabel?: string;
  sourceLabel?: string;
  sourceCustomerLabel?: string;
  paymentForLabel?: string;
  paymentForClass?: string;
  serviceInfo?: ServiceModel;
  countryInfo?: LocationCountryModel;
  revenue?: UserProfileModel;
  doctorRevenue?: UserProfileModel;
  createdByInfo?: UserProfileModel;
  updatedByInfo?: UserProfileModel;

  // Nhóm BHYT (cột DB, thêm sau baseline) — màn chi tiết hiển thị phần bảo hiểm chi trả
  emr_insurance_code?: string;
  emr_insurance_ratio?: number;
  emr_insurance_amount?: number;
  emr_patient_amount?: number;
  customerDisplayName?: string;
  paymentForKey?: string; // labelKey i18n của loại phiếu (thay cho paymentForLabel hardcode)
  payment_timeFormatted?: string;
  created_atFormatted?: string;
  updated_atFormatted?: string;
  // Quan hệ + phân bổ do endpoint `/:id/detail` trả (xem docblock GetPaymentDetail bên BE)
  orderPlan?: OrderPlanModel;
  serviceUsed?: OrderServiceUsedModel;
  appointment?: AppointmentModel;
  /**
   * Các phiếu thu CÙNG ĐƠN HÀNG. Hệ thống không có bảng phân bổ riêng: mỗi dịch vụ được thu thành
   * MỘT dòng `payment` riêng (mã UNIQUE), nên "phân bổ" chính là tập phiếu của đơn đó.
   */
  allocations?: PaymentModel[];
  deposit?: PaymentModel;
  total?: number;
  orderServiceUsed?: OrderServiceUsedModel; // Thanh toán cho dịch vụ nào
  branch?: BranchModel;
  source?: SourceModel;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  treatmentPlan?: OrderPlanModel;
  paymentMethod?: PaymentMethodModel;
  customer?: CustomerModel;
  cardType?: CardTypeModel;
}
