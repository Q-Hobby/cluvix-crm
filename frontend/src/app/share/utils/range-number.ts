// utils/range.ts
import { NgIterable } from '@angular/core';

export function rangeNumber(count: number): NgIterable<number> {
  const n = Number.isFinite(count) ? Math.max(0, Math.trunc(count)) : 0;
  return {
    [Symbol.iterator](): Iterator<number> {
      let i = 0;
      return {
        next(): IteratorResult<number> {
          return i < n ? { value: i++, done: false } : { value: undefined as any, done: true };
        },
      };
    },
  };
}
