import {Service, signal, WritableSignal } from '@angular/core';

@Service()
export class GlobalSearchService {
  readonly visible: WritableSignal<boolean> = signal(false);

  open = () => this.visible.set(true);
  close = () => this.visible.set(false);

  toggle(force: boolean | undefined = undefined) {
    force === undefined ? this.visible.update((v) => !v) : this.visible.set(force);
  }
}
