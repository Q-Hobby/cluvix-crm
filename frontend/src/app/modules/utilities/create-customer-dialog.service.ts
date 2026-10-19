import { inject, Service} from '@angular/core';
import { DynamicDialogRef } from 'primeng/dynamicdialog';
import { TranslateService } from '@ngx-translate/core';
import { CustomerFormV2 } from '@/modules/crm/customer/form/form-customer/customer-form-v2';
import { DialogWrapperService } from '@/share/services/dialog-wrapper.service';
import { Router } from '@angular/router';

@Service()
export class CreateCustomerDialogService {
  private translate = inject(TranslateService);
  protected dialogWrapperService: DialogWrapperService = inject(DialogWrapperService);
  protected route: Router = inject(Router);
  private ref?: DynamicDialogRef;

  open() {
    if (this.ref) return; // đã mở rồi thì thôi (tránh trùng)
    this.ref = this.dialogWrapperService.open(CustomerFormV2, {
      header: this.translate.instant('utilities.dialog_create'),
      width: '700px',
    });

    this.ref.onClose.subscribe((result: any) => {
      if (result) {
        // Truyền customer qua state để detail page paint instant + tránh tab con
        // (VD payment) gọi API thiếu customer_id trong lúc chờ GET model resolve.
        this.route.navigate([`/customer/detail/${result.id}`], { state: { customer: result } });
      }
    });
    this.ref.onDestroy?.subscribe(() => (this.ref = undefined));
  }

  close() {
    this.ref?.close();
    this.ref = undefined;
  }

  toggle() {
    this.ref ? this.close() : this.open();
  }
}
