import {
  Directive,
  Input,
  TemplateRef,
  ViewContainerRef,
  effect,
  inject,
  signal,
} from '@angular/core';

import { isSuperUser } from '@/core/services/auth/auth.selectors';
import { userPermissions } from '@/core/services/auth/auth.state';

/**
 * Render khi user có permission. Super role bypass.
 *
 * Usage:
 *   <button *hasPerm="'createCustomer'">Create</button>
 *   <button *hasPerm="['createCustomer','updateCustomer']">Edit</button>  // OR
 */
@Directive({
  selector: '[hasPerm]',
  standalone: true,
})
export class HasPermDirective {
  private vcr = inject(ViewContainerRef);
  private tpl = inject(TemplateRef);
  private permValue = signal<string | string[]>([]);

  @Input({ required: true }) set hasPerm(value: string | string[]) {
    this.permValue.set(value);
  }

  constructor() {
    effect(() => {
      const value = this.permValue();
      let ok = isSuperUser();
      if (!ok) {
        const set = userPermissions();
        const perms = Array.isArray(value) ? value : [value];
        ok = perms.some((p) => set.has(p));
      }
      this.vcr.clear();
      if (ok) this.vcr.createEmbeddedView(this.tpl);
    });
  }
}
