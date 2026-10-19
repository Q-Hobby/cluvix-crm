import {Service, signal } from '@angular/core';

import { CustomerModel } from '@/core/models/crm/customer.model';

export interface PrintContext {
  branch?: number;
  customer?: CustomerModel;
  layoutType?: string;
  doctorName?: string;
}

@Service()
export class PrintDataService {
  private readonly _model = signal<PrintContext | null>(null);
  readonly model = this._model.asReadonly();

  setModel(data: PrintContext) {
    this._model.set(data);
  }

  clear() {
    this._model.set(null);
  }
}
