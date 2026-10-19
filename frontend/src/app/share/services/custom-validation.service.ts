import { inject, Service} from '@angular/core';
import { AbstractControl, ValidatorFn } from '@angular/forms';

import { UtilsService } from '@/share/services/utils.service';

@Service()
export class CustomValidationService {
  protected utilsService = inject(UtilsService);

  public url(): ValidatorFn {
    return (control: AbstractControl) => {
      if (!control || !control.value) {
        return null;
      }

      if (!this.utilsService.isUrl(control.value)) {
        return { url: true };
      }

      return null;
    };
  }

  public email(): ValidatorFn {
    return (control: AbstractControl) => {
      if (!control || !control.value) {
        return null;
      }

      if (!this.utilsService.isEmail(control.value)) {
        return { email: true };
      }

      return null;
    };
  }
}

export function greaterThanNumber(num: number): ValidatorFn {
  return (control: AbstractControl) => {
    if (!control || control.value === null) {
      return null;
    }

    if (control.value > num) {
      return null;
    }

    return { greaterThanNumber: num };
  };
}

export function lessThanOrEqual(num: number): ValidatorFn {
  return (control: AbstractControl) => {
    if (!control || control.value === null) {
      return null;
    }

    if (control.value <= num) {
      return null;
    }

    return { lessThanOrEqual: num };
  };
}
