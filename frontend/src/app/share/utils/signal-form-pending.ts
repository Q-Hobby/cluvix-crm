import { computed, Signal, signal } from '@angular/core';

/**
 * Theo dõi "form có thay đổi CHƯA LƯU hay không" cho form Signal Forms.
 *
 * **Vì sao cần**: bản Reactive Forms đạt việc này bằng `valueChanges` + patch giá trị kỹ thuật với
 * `{ emitEvent: false }` (vd patch lại `version` sau khi lưu mà không bật cờ dirty). Signal Forms
 * **KHÔNG có `emitEvent`** — ghi signal là luôn notify, nên cách "bắt sự kiện thay đổi" sẽ tự bật lại
 * cờ ngay sau mỗi lần lưu và gây gọi API thừa (hoặc vòng lặp nếu có autosave).
 *
 * **Cách làm**: đảo chiều — không bắt sự kiện, mà so **chữ ký của tập field NGHIỆP VỤ** giữa hiện tại
 * và lần lưu gần nhất. Field kỹ thuật (`version`, `id`, …) cố ý KHÔNG nằm trong `businessKeys` nên
 * patch chúng không bao giờ làm form "bẩn" trở lại.
 *
 * ```ts
 * private readonly pending = createPendingTracker(this.model, CLINICAL_BUSINESS_KEYS);
 * readonly hasPendingChanges = this.pending.hasPendingChanges;
 * // sau khi seed xong và sau MỖI lần lưu thành công:
 * this.pending.markSaved();
 * ```
 *
 * ⚠ `businessKeys` phải liệt kê theo THỨ TỰ CỐ ĐỊNH và hàm seed model phải dựng object lồng với thứ
 * tự khoá ổn định — chữ ký so bằng `JSON.stringify`, đổi thứ tự khoá là đổi chữ ký.
 */
export function createPendingTracker<T extends object>(
  model: Signal<T>,
  businessKeys: readonly (keyof T)[],
) {
  const signature = computed(() => JSON.stringify(businessKeys.map((k) => model()[k] ?? null)));
  const savedSignature = signal(signature());

  return {
    /** True khi giá trị nghiệp vụ hiện tại khác lần `markSaved()` gần nhất. */
    hasPendingChanges: computed(() => signature() !== savedSignature()),
    /** Đánh dấu "trạng thái hiện tại đã được lưu" — gọi sau khi seed và sau mỗi lần lưu thành công. */
    markSaved: () => savedSignature.set(signature()),
  };
}
