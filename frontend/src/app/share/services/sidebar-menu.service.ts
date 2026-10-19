import {Service, signal } from '@angular/core';

@Service()
export class SidebarMenuService {
  private _menu = signal<any[]>([]);
  public menu = this._menu.asReadonly();

  setMenu(menu: any[]) {
    this._menu.set(menu);
  }
}
