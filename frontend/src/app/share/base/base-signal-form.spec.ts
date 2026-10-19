import { DestroyRef, signal } from '@angular/core';
import { of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';

import { BaseSignalForm } from './base-signal-form';

/**
 * Story-61 (E6 bỏ company khỏi IndexedDB cache + base form + form-control): base-signal-form.ts đã bỏ
 * toàn bộ khuôn "seed/guard/offer company_id" (`seedCompanyId`, `guardCompanyBeforeSubmit`,
 * `offerAssignSharedRecord`, `warnIfCompanyMissingOnCreate`, `isCompanyFieldRequired`,
 * `warnCompanyMissing`, `currentCompanyId`, `watchSharedRecordOffer`) — khoá hành vi cũ của epic
 * `fe-remove-company-picker` (story-01) không còn áp dụng, xoá toàn bộ spec cũ về nhóm này (~700 dòng).
 * File này chỉ còn giữ lại phần TỔNG QUÁT vẫn đúng sau khi bỏ company: hook `prepareModel()` chạy
 * trong `ngOnInit()` ở cả 3 nhánh (edit GET, create trống, create prefill `data.model`).
 */
type HookModel = { id?: number; name?: string };

const ngOnInit = BaseSignalForm.prototype.ngOnInit as (this: unknown) => void;

function makeHost(opts: {
  id?: number | null;
  dialogModel?: Partial<HookModel> | null;
  initialValue?: Partial<HookModel>;
  loaded?: Partial<HookModel>;
}) {
  const seen: HookModel[] = [];
  const model = signal<HookModel>((opts.dialogModel as HookModel) ?? ({} as HookModel));
  const host = {
    id: signal<number | null>(opts.id ?? null),
    model,
    initialValue: opts.initialValue ?? {},
    destroyRef: { onDestroy: () => () => undefined } as unknown as DestroyRef,
    dialogConfig: { data: { model: opts.dialogModel ?? undefined } },
    modelService: {
      getInitialObject: () => ({ name: 'default' }) as HookModel,
      get: () => of({ id: opts.id, ...opts.loaded } as HookModel),
    },
    prepareModel: (obj: HookModel) => {
      seen.push(obj);
      return { ...obj, name: 'hooked' };
    },
  };
  return { host, seen, model };
}

describe('BaseSignalForm.prepareModel (hook trong ngOnInit)', () => {
  it('nhánh create trống → hook được gọi 1 lần với default + initialValue, model.set nhận giá trị hook trả về', () => {
    const { host, seen, model } = makeHost({ initialValue: { name: 'seed' } });

    ngOnInit.call(host);

    expect(seen).toHaveLength(1);
    expect(model().name).toBe('hooked');
  });

  it('nhánh create có prefill `data.model` → hook nhận default merge model prefill', () => {
    const { host, seen, model } = makeHost({ dialogModel: { id: undefined, name: 'prefill' } });

    ngOnInit.call(host);

    expect(seen).toHaveLength(1);
    expect(model().name).toBe('hooked');
  });

  it('nhánh edit → hook gọi 2 lần: set khởi tạo (default) rồi set sau GET (default + loaded)', () => {
    const { host, seen, model } = makeHost({ id: 3, loaded: { name: 'từ BE' } });

    ngOnInit.call(host);

    expect(seen).toHaveLength(2);
    expect(seen[0]).toEqual({ name: 'default' });
    expect(seen[1]).toEqual({ id: 3, name: 'từ BE' });
    expect(model().name).toBe('hooked');
  });

  it('mặc định (không override) là no-op — trả về đúng object nhận vào', () => {
    const obj = { id: 1, name: 'x' } as HookModel;

    const out = (
      BaseSignalForm.prototype as unknown as { prepareModel(o: HookModel): HookModel }
    ).prepareModel.call({}, obj);

    expect(out).toBe(obj);
  });
});

describe('BaseSignalForm.onSubmit — không còn gate company_id', () => {
  it('form invalid → không markAsTouched()/submit dừng sớm (guard company đã bỏ, chỉ còn validate chuẩn)', () => {
    const markAsTouched = vi.fn();
    const submitted = signal(false);
    const isSubmitting = signal(false);
    const host = {
      isSubmitting,
      submitted,
      rootField: () => ({ markAsTouched, invalid: () => true }),
    };

    (BaseSignalForm.prototype.onSubmit as (this: unknown) => void).call(host);

    expect(markAsTouched).toHaveBeenCalledTimes(1);
    expect(submitted()).toBe(true);
  });
});
