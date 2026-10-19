import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { UserProfileModel } from '@/core/models/user-profile.model';

/** AppointmentModel đã qua addLabel() cho report "Lịch hẹn tư vấn" — thêm display fields
 * resolve từ IndexedDB cache (KHÔNG phải BE trả kèm — xem architecture.md FR7).
 * `branchLabel`/`appointmentTypeLabel`/`appointment_dateFormatted` đã khai báo sẵn trên
 * `AppointmentModel` — chỉ thêm field còn thiếu ở đây.
 *
 * `doctorInfo`/`doctorAppointmentInfo`/`directSalesInfo` đã BỎ (story-53, D19) — BE story-46 đã bỏ
 * `doctor_id`/`doctorAppointment_id`/`direct_sales_id` khỏi response report này, story-51 bỏ theo khỏi
 * `AppointmentModel`, story-53 xoá luôn field + cột UI hiển thị.
 */
export interface AppointmentConsultationDisplayModel extends AppointmentModel {
  serviceTypeAcceptedLabel?: string;
  appointmentReasonLabel?: string;
  createdByInfo?: UserProfileModel | null;
  checkin_dateFormatted?: string;
  checkout_dateFormatted?: string;
  created_atFormatted?: string;
}
