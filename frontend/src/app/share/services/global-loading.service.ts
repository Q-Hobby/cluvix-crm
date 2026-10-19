import { Service, signal, Signal, WritableSignal } from '@angular/core';
import { Observable } from 'rxjs';
import { finalize } from 'rxjs/operators';

import { resolveLoadingLabelKeyFromKey } from '@/core/interceptors/loading-label.util';

@Service()
export class GlobalLoadingService {
  private loadingMap = new Map<string, number>();
  private totalCount: WritableSignal<number> = signal(0); // signal lưu tổng số loading

  // Danh sách các lượt loading đang active (song song với loadingMap) — mỗi lượt showLoading
  // push 1 entry, hideLoading xoá đúng entry cuối cùng khớp key (LIFO). Chỉ dùng để suy ra
  // labelKey của lượt active gần nhất; KHÔNG thay thế cơ chế đếm loadingMap/totalCount.
  private activeEntries: Array<{ key: string; labelKey?: string }> = [];
  private readonly _labelKey: WritableSignal<string | undefined> = signal(undefined);

  // ✅ Public signal: labelKey (i18n key) của lượt loading active gần nhất — undefined nếu không có.
  public readonly labelKey: Signal<string | undefined> = this._labelKey.asReadonly();

  // Chỉ hiện spinner nếu loading kéo dài hơn ngưỡng này — tránh chớp nháy với API nhanh.
  private readonly SHOW_DELAY_MS = 150;
  // Khi đã hiện, giữ tối thiểu ngần này trước khi ẩn — tránh vừa hiện đã biến mất.
  private readonly MIN_VISIBLE_MS = 250;

  private showTimer: ReturnType<typeof setTimeout> | null = null;
  private hideTimer: ReturnType<typeof setTimeout> | null = null;
  private shownAt = 0;

  private readonly _visible: WritableSignal<boolean> = signal(false);

  // ✅ Public signal để các component dùng — debounced, không phản ánh trực tiếp totalCount.
  public readonly isLoading: Signal<boolean> = this._visible.asReadonly();

  /**
   * Show loading với key. `labelKey` (i18n key) là optional — không truyền vẫn hoạt động y hệt cũ.
   */
  showLoading(key: string, labelKey?: string): void {
    const current = this.loadingMap.get(key) ?? 0;
    this.loadingMap.set(key, current + 1);
    // Không truyền labelKey thì thử suy từ chính key. Cần cho export `?_export=1`: đó là request
    // GET nên loadingInterceptor không gắn label, overlay lại do component bật bằng
    // showLoading('exportXxx') — không có bước này thì loader ghi "Đang tải..." thay vì "Đang xuất file".
    this.activeEntries.push({ key, labelKey: labelKey ?? resolveLoadingLabelKeyFromKey(key) });
    this.updateLabel();
    this.updateTotal();
  }

  /**
   * Hide loading theo key. Xoá đúng entry cuối cùng khớp key trong activeEntries (LIFO),
   * không đổi logic đếm loadingMap.
   */
  hideLoading(key: string): void {
    const current = this.loadingMap.get(key) ?? 1;
    if (current <= 1) this.loadingMap.delete(key);
    else this.loadingMap.set(key, current - 1);
    const idx = this.findLastEntryIndex(key);
    if (idx !== -1) this.activeEntries.splice(idx, 1);
    this.updateLabel();
    this.updateTotal();
  }

  /**
   * Wrap một Observable và tự show/hide loader theo key. `labelKey` optional truyền xuống showLoading.
   */
  track<T>(key: string, stream$: Observable<T>, labelKey?: string): Observable<T> {
    this.showLoading(key, labelKey);
    return stream$.pipe(finalize(() => this.hideLoading(key)));
  }

  /**
   * Cập nhật labelKey signal = labelKey của entry cuối cùng (mới nhất) còn active — undefined nếu rỗng.
   */
  private updateLabel(): void {
    const last = this.activeEntries[this.activeEntries.length - 1];
    this._labelKey.set(last?.labelKey);
  }

  /**
   * Tìm index của entry cuối cùng khớp key (duyệt ngược). Viết tay thay vì Array.prototype.findLastIndex
   * vì target ES2022 chưa có sẵn API ES2023 trong lib mặc định.
   */
  private findLastEntryIndex(key: string): number {
    for (let i = this.activeEntries.length - 1; i >= 0; i--) {
      if (this.activeEntries[i].key === key) return i;
    }
    return -1;
  }

  /**
   * Tổng hợp lại số lượng loading để cập nhật signal
   */
  private updateTotal(): void {
    const total = Array.from(this.loadingMap.values()).reduce((a: any, b: any) => a + b, 0);
    this.totalCount.set(total);
    this.scheduleVisibility(total > 0);
  }

  private scheduleVisibility(shouldShow: boolean): void {
    if (shouldShow) {
      if (this._visible() || this.showTimer) return;
      if (this.hideTimer) {
        clearTimeout(this.hideTimer);
        this.hideTimer = null;
      }
      this.showTimer = setTimeout(() => {
        this.showTimer = null;
        this._visible.set(true);
        this.shownAt = Date.now();
      }, this.SHOW_DELAY_MS);
      return;
    }

    if (this.showTimer) {
      clearTimeout(this.showTimer);
      this.showTimer = null;
      return; // Chưa kịp hiện — không cần làm gì thêm.
    }
    if (!this._visible() || this.hideTimer) return;

    const remaining = Math.max(0, this.MIN_VISIBLE_MS - (Date.now() - this.shownAt));
    this.hideTimer = setTimeout(() => {
      this.hideTimer = null;
      this._visible.set(false);
    }, remaining);
  }
}
