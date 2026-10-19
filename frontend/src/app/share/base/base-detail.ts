import { DestroyRef, Directive, inject, signal } from '@angular/core';
import { Title } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { finalize } from 'rxjs';

import { BaseApp } from '@/share/base/BaseApp';
import { BaseApiService } from '@/share/services/base-api.service';

@Directive()
export abstract class BaseDetail<T extends { id?: number }> extends BaseApp {
  abstract modelService: BaseApiService<T>;
  readonly id = signal<number | null>(null);
  readonly model = signal<T>({} as T);
  readonly resetForm = signal<number | null>(null);
  protected readonly title = inject(Title);
  protected readonly activatedRoute = inject(ActivatedRoute);
  protected destroyRef = inject(DestroyRef);

  getModel = (id: number) => {
    this.loading.set(true);
    this.modelService
      .get(id, {})
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (model: any) => {
          this.model.set(model);
          if (model?.id) {
            this.resetForm.set(model.id);
          }
          this.afterModelLoaded(model);
        },
        error: (err) => this.handleModelLoadError(err),
      });
  };

  findValue(idValue: any, objectArray: any) {
    return objectArray.find((x: any) => x.value == idValue)?.label;
  }

  /**
   * Hook được gọi sau khi model load thành công
   * Override trong class con để xử lý logic riêng
   */
  protected afterModelLoaded(model: T): void {
    // Override in child class if needed
  }

  /**
   * Hook xử lý lỗi khi load model
   * Override trong class con để xử lý logic riêng
   */
  protected handleModelLoadError(err: any): void {
    console.error('Error loading model:', err);
  }
}
