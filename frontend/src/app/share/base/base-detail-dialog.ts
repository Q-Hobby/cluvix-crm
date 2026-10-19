import { inject, Service, signal } from '@angular/core';
import { Title } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';

import { BaseApp } from '@/share/base/BaseApp';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export abstract class BaseDetailDialog<T extends { id?: number }> extends BaseApp {
  title = inject(Title); // Optional nếu bạn muốn cập nhật title browser cho trang chi tiết.
  readonly resetForm = signal<number | null>(null);
  id = signal<number | null>(null);
  model = signal<T>({} as T);
  abstract modelService: BaseApiService<T>;
  protected activatedRoute: ActivatedRoute = inject(ActivatedRoute);
  protected dialogConfig: DynamicDialogConfig = inject(DynamicDialogConfig);

  protected constructor() {
    super();
    const modelParam = structuredClone(this.dialogConfig?.data?.model);
    this.model.set(modelParam);
  }

  /**
   * Hàm tìm kiếm lable theo value
   * @param idValue
   * @param objectArray
   */
  findValue(idValue: any, objectArray: any) {
    return objectArray.find((x: any) => x.value == idValue)?.label;
  }

  closeDialog() {
    this.dialogRef.close(false);
  }
}
