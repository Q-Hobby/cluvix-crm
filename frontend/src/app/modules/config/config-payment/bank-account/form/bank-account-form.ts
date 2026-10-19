import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';
import { finalize } from 'rxjs';

import { BankAccountModel } from '@/core/models/config/accountant/bank-account.model';
import { BankAccountService } from '@/core/services/config/accountant/bank-account.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-bank-account-form',
  templateUrl: './bank-account-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class BankAccountForm extends BaseSignalForm<BankAccountModel> {
  modelService = inject(BankAccountService);

  override onSubmit(): void {
    if (this.isSubmitting()) return;

    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    const payload = this.getPayload();
    this.isSubmitting.set(true);
    let responseData: any = null;

    const handleSuccess = (res: any) => {
      responseData = res;
      this.noti.successMsg(
        this.model().id
          ? res.message || this.translate.instant('common.message.update_success')
          : this.translate.instant('common.message.create_success'),
      );
    };

    const handleFinalize = () => {
      this.isSubmitting.set(false);
      if (responseData) {
        this.dialogRef.close(this.model().id ? responseData.data : responseData);
      }
    };

    if (this.model().id && !this.dialogConfig?.data?.clone) {
      this.modelService
        .updateBankAccount(this.model().id!, payload)
        .pipe(finalize(handleFinalize))
        .subscribe({ next: handleSuccess });
    } else {
      this.modelService
        .createBankAccount(payload)
        .pipe(finalize(handleFinalize))
        .subscribe({ next: handleSuccess });
    }
  }

  protected override buildSchema(p: SchemaPathTree<BankAccountModel>): void {
    required(p.bank_code!);
    required(p.account_no!);
    required(p.account_name!);
  }
}
