import { Identifiable } from '@/core/interface/identifiable';
import { AppointmentReasonModel } from '@/core/models/config/appointment/appointment-reason.model';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { ServiceTypeModel } from '@/core/models/config/clinic/service-type.model';
import { SourceModel } from '@/core/models/config/marketing/source.model';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { OrderServiceUsedModel } from '@/core/models/crm/order-service-used.model';
import { UserProfileModel } from '@/core/models/user-profile.model';
import { ServiceModel } from '@/core/models/config/clinic/service.model';
import { appointmentStatusInfo, confirmAppointmentInfo } from '@/define';

export interface AppointmentModel extends Identifiable {
  id?: number;
  customer_id?: number; // ID khách hàng
  code?: string; // Mã lịch hẹn
  branch_id?: number; // Chi nhánh
  appointment_date_first?: number; // Thời gian hẹn lần đầu (timestamp)
  appointment_date?: any; // Thời gian hẹn hiện tại (timestamp)
  appointmentListDate?: any; // Lịch sử thay đổi ngày hẹn (JSON array)
  /** Thời gian khách đến. // ⚠ SAU afterLoad là `Date`, không còn là số giây (BaseApiService.unixTimesTampFields) */
  checkin_date?: number;
  /** Thời gian khách về. // ⚠ SAU afterLoad là `Date`, không còn là số giây (BaseApiService.unixTimesTampFields) */
  checkout_date?: number;
  noteTimeline?: any[]; // Timeline tương tác với khách (JSON array)
  noteAppointment?: string; // Ghi chú lịch hẹn
  source_id?: number; // Nguồn khách hàng theo lịch hẹn
  appointment_type_id?: number; // Loại lịch hẹn
  schedule_time?: number; // Thời gian dự kiến (phút)
  appointment_number?: number; // Tổng số lần hẹn
  content?: string; // Nội dung lịch hẹn (HTML format)
  contentPrivate?: string; // Ghi chú nội bộ nhân viên
  scheduleTime?: number; // Thời gian dự kiến tính toán
  order_service_used_id?: number; // Dịch vụ điều trị
  type_service_id?: number; // Nhóm dịch vụ quan tâm
  service_id?: number; // Dịch vụ hẹn
  is_accepted?: number; // Khách hàng đồng ý làm dịch vụ hay không (1=Đồng ý, 2=Không đồng ý)
  confirmAppointment?: number; // Xác nhận lịch (0=Chưa xác nhận, 1=Đã xác nhận)
  status?: any; // Tình trạng lịch hẹn (1=Chưa đến, 2=Đã đến, 3=Ra về, 4=Hủy, 5=Đang tư vấn, 6=Đang điều trị)
  statusAppointment?: string; // Tên trạng thái lịch hẹn
  appointment_reason_id?: number; // Lý do
  revenue_id?: number; // Người tư vấn
  convert_id?: number; // Người chuyển đổi
  type_service_accepted_id?: number; // Nhóm dịch vụ đồng ý
  bookType?: number; // Phân loại lịch hẹn (1=Đặt trước, 2=Phát sinh, 3=Dự kiến)
  customer_data?: any[]; // Control rỗng giữ lại từ form gốc (không dùng ở payload, không có getter/setter riêng)

  // EMR Fields - Thông tin y tế
  emr_department_id?: number; // Khoa khám (FK → emr_department)
  emr_room_id?: number; // Phòng khám (FK → emr_room)
  emr_queue_number?: number; // Số thứ tự khám
  emr_weight?: number; // Cân nặng (kg)
  emr_height?: number; // Chiều cao (cm)
  emr_bmi?: number; // BMI (chỉ số khối cơ thể, tự tính)
  emr_blood_pressure?: string; // Huyết áp (mmHg) - VD: 120/80
  emr_pulse?: number; // Mạch (đập/phút)
  emr_temperature?: number; // Nhiệt độ (°C)
  emr_respiratory_rate?: number; // Nhịp thở (lần/phút)
  emr_spo2?: number; // SpO2 - Độ bão hòa oxy trong máu (%)
  emr_chief_complaint?: string; // Lý do khám - Triệu chứng chính
  emr_symptoms?: string; // Triệu chứng chi tiết
  emr_medical_history?: string; // Tiền sử bệnh (sơ bộ khi tiếp nhận)
  emr_medical_record_id?: number; // Bệnh án (FK → emr_medical_record, sau khi khám)

  created_by?: number; // Người tạo
  updated_by?: number; // Người cập nhật
  created_at?: number; // Ngày tạo (timestamp)
  updated_at?: number; // Ngày cập nhật (timestamp)

  // Computed/Display Fields
  confirmAppointmentInfo?: confirmAppointmentInfo;
  customerDisplayName?: string;
  // Nhãn + thông tin dựng ở FE bởi `AppointmentService.addLabel` (cache IndexedDB) — không phải cột DB.
  genderLabel?: string;
  genderKey?: string;
  provinceLabel?: string;
  sourceLabel?: string;
  serviceTypeAcceptedLabel?: string;
  bookTypeInfo?: { value: number; label: string; labelKey?: string };
  appointmentIsAcceptedInfo?: { name: string; severity: string } | null;
  appointmentReasonInfo?: { id?: number; name?: string } | null;
  serviceInfo?: ServiceModel | null;
  createdByInfo?: UserProfileModel | null;
  updatedByInfo?: UserProfileModel | null;
  convertInfo?: UserProfileModel | null;
  paymentUser?: UserProfileModel | null;
  // Chuỗi ngày giờ đã format sẵn (`addFormattedTimestampFields`)
  appointment_date_firstFormatted?: string;
  checkin_dateFormatted?: string;
  checkout_dateFormatted?: string;
  created_atFormatted?: string;
  branchLabel?: string;
  appointmentTypeLabel?: string;
  appointment_dateFormatted?: string;
  serviceTypeLabel?: string;
  statusInfo?: appointmentStatusInfo;

  // Relations
  orderServiceUsed?: OrderServiceUsedModel;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  customer?: CustomerModel;
  appointmentType?: AppointmentTypeModel;
  branch?: BranchModel;
  owner?: UserProfileModel;
  revenue?: UserProfileModel;
  ticket?: UserProfileModel;
  serviceType?: ServiceTypeModel;
  service?: ServiceModel;
  typeServiceInterested?: ServiceTypeModel;
  appointmentReason?: AppointmentReasonModel;
  source?: SourceModel;

  /** Cảnh báo không chặn (D6) — vd gắn đợt lỗi nhưng check-in vẫn thành công. */
  warnings?: string[];

  // Calendar Fields
  start?: Date;
  end?: Date;
  title?: string;
  backgroundColor?: string;
  borderColor?: string;
  extendedProps?: AppointmentModel;
  notesTotal?: number;
  statusName?: string;
  type?: number;
}
