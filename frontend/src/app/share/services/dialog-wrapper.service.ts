import { inject, Service, Type } from '@angular/core';
import { fromEvent } from 'rxjs';
import { take } from 'rxjs/operators';
import { DialogService, DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';
import { DIALOG_RESPONSIVE_BREAKPOINTS } from '@/define';

@Service()
// Wrapper lại Service Dialog
export class DialogWrapperService {
  private dialogService = inject(DialogService);

  open(component: Type<any>, config: Partial<DynamicDialogConfig> = {}): DynamicDialogRef {
    const defaultConfig: DynamicDialogConfig = {
      closable: true,
      maximizable: true,
      dismissableMask: true,
      closeOnEscape: true,
      modal: true,
      autoZIndex: true,
      focusOnShow: false, // Prevent auto-focus on first input
      styleClass: '',
      width: '60%',
      breakpoints: DIALOG_RESPONSIVE_BREAKPOINTS,
      ...config,
    };

    const ref = this.dialogService.open(component, defaultConfig)!;
    this.bindBackButton(ref);
    return ref;
  }

  private bindBackButton(ref: DynamicDialogRef): void {
    history.pushState({ dialog: true }, '');

    const popstateSub = fromEvent<PopStateEvent>(window, 'popstate')
      .pipe(take(1))
      .subscribe(() => ref.close());

    ref.onClose.pipe(take(1)).subscribe(() => {
      popstateSub.unsubscribe();
      // Dọn history entry giả bằng replaceState (không phải history.back()) —
      // back() là async và có thể pop nhầm entry mà router.navigate() vừa push
      // nếu caller điều hướng ngay sau khi dialog đóng (VD: tạo mới xong vào trang detail).
      if (history.state?.dialog) {
        history.replaceState(null, '', location.href);
      }
    });
  }

  openForm(component: Type<any>, title: string, data?: any): DynamicDialogRef {
    return this.open(component, {
      header: title,
      width: '700px',
      data,
    });
  }

  openPreview(component: Type<any>, title: string, data?: any): DynamicDialogRef {
    return this.open(component, {
      header: title,
      width: '70vw',
      styleClass: 'p-dialog-preview',
      data,
    });
  }

  openConfirm(
    component: Type<any>,
    message: string,
    onConfirm: () => void,
    onCancel?: () => void,
  ): void {
    const ref = this.open(component, {
      header: 'Xác nhận',
      width: '400px',
      data: { message },
    });

    ref.onClose.subscribe((result: any) => {
      if (result === true && onConfirm) onConfirm();
      else if (onCancel) onCancel();
    });
  }
}
