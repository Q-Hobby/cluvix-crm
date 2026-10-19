import { Service} from '@angular/core';



@Service()
export class UtilsService {
  readonly mappingFilter = {
    contains: 'like', //So sánh từ khóa có tồn tại
    equals: 'eq',
    in: 'in',
    gt: 'gt',
    lt: 'lt',
    lte: 'lte',
    gte: 'gte',
    neq: 'neq',
    nin: 'nin',
    and: 'and',
    or: 'or',
    not: 'not',
    is: 'is',
    nis: 'nis',
  };

  constructor() {}

  buildFilter(filter: any) {
    const fields = Object.keys(filter),
      result = {};

    fields.forEach((value: any) => {
      // p-table clearFilterValues() chỉ set filterMetadata.value = null (không xoá key) —
      // filter còn lại 'in'/'between' với value null sẽ throw khi gọi .forEach/[0]/[1] bên dưới,
      // làm chết luôn rxjs subscription (valueChanges) nên các lần filter sau đó im lặng không chạy.
      if (!filter[value] || this.isFilterValueBlank(filter[value].value)) {
        return;
      }
      if (value === '_keyword') {
        result['_keyword'] = filter[value].value;
      } else {
        // Tìm tất cả các filter bắt đầu bằng _
        if (value.startsWith('_')) {
          if (filter[value].matchMode === 'in') {
            filter[value].value.forEach((filterValue: any, index: any) => {
              result[`${value}[${index}]`] = filterValue;
            });
          } else if (filter[value].matchMode === 'custom') {
            result[`${value}`] = filter[value].value;
          } else if (filter[value].matchMode === 'between') {
            result[`${value}[gte]`] = filter[value].value[0];
            result[`${value}[lte]`] = filter[value].value[1];
          } else {
            result[`${value}[${this.mappingFilter[filter[value].matchMode]}]`] =
              filter[value].value;
          }
        } else {
          if (filter[value].matchMode === 'in') {
            filter[value].value.forEach((filterValue: any, index: any) => {
              result[`filter[${value}][${index}]`] = filterValue;
            });
          } else if (filter[value].matchMode === 'between') {
            result[`filter[${value}][gte]`] = filter[value].value[0];
            result[`filter[${value}][lte]`] = filter[value].value[1];
          } else if (filter[value].matchMode === 'custom') {
            // BE đọc raw c.Query(field) thay vì query.ApplyFilter — gửi thẳng field=value,
            // không bọc filter[field][...]. Cùng luật với nhánh field bắt đầu bằng `_` ở trên.
            result[value] = filter[value].value;
          } else {
            result[`filter[${value}][${this.mappingFilter[filter[value].matchMode]}]`] =
              filter[value].value;
          }
        }
      }
    });

    return result;
  }

  private isFilterValueBlank(value: any): boolean {
    if (value === null || value === undefined) return true;
    if (typeof value === 'string' && value.trim().length === 0) return true;
    if (Array.isArray(value) && value.length === 0) return true;
    return false;
  }

  isPositiveInt(str: string): boolean {
    return !/\D/.test(str);
  }

  isUrl(url: string): boolean {
    const pattern = new RegExp(
      '^(https?:\\/\\/)?' + // protocol
        '((([a-z\\d]([a-z\\d-]*[a-z\\d])*)\\.)+[a-z]{2,}|' + // domain name
        '((\\d{1,3}\\.){3}\\d{1,3}))' + // OR ip (v4) address
        '(\\:\\d+)?(\\/[-a-z\\d%_.~+]*)*' + // port and path
        '(\\?[;&a-z\\d%_.~+=-]*)?' + // query string
        '(\\#[-a-z\\d_]*)?$',
      'i',
    ); // fragment locator

    return !!pattern.test(url);
  }

  isEmail(email: string): boolean {
    const re =
      /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
    return !!re.test(String(email).toLowerCase());
  }

  isIdentityCard(identityCard: string): boolean {
    return identityCard.length === 9 || identityCard.length === 12;
  }

  stringContainsOnlyNumber(identityCard: string): boolean {
    return /^\d+$/.test(identityCard);
  }

  randomNumber(min: any, max: any): number {
    min = Math.ceil(min);
    max = Math.floor(max);
    return Math.floor(Math.random() * (max - min) + min); // The maximum is exclusive and the minimum is inclusive
  }

  toUnixSeconds(
    date: Date | string | null | undefined,
    mode: 'start' | 'end' = 'end',
  ): number | undefined {
    if (!date) return undefined;
    const d = new Date(date); // p-datePicker thường trả Date; nếu là string thì Date(...) vẫn parse được nếu là ISO
    if (Number.isNaN(d.getTime())) return undefined;

    if (mode === 'start') {
      d.setHours(0, 0, 0, 0);
    } else {
      d.setHours(23, 59, 59, 999);
    }
    // UNIX seconds (MySQL INT)
    return Math.floor(d.getTime() / 1000);
  }

  randomString(length = 10): string {
    const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    const cryptoObj = (window as any).crypto;
    if (cryptoObj?.getRandomValues) {
      const arr = new Uint32Array(length);
      cryptoObj.getRandomValues(arr);
      let out = '';
      for (let i = 0; i < length; i++) out += alphabet[arr[i] % alphabet.length];
      return out;
    }
    // Fallback không crypto
    let out = '';
    for (let i = 0; i < length; i++) {
      out += alphabet[Math.floor(Math.random() * alphabet.length)];
    }
    return out;
  }
}

export function getCurrentDateInTimeStamp() {
  const d = new Date(); d.setHours(0,0,0,0); return Math.floor(d.getTime() / 1000);
}

export function toFormData<T extends object>(
  formValue: T,
  parentKey?: string,
  formData: FormData = new FormData(),
): FormData {
  for (const key of Object.keys(formValue)) {
    const value = formValue[key];
    const formKey = parentKey ? `${parentKey}[${key}]` : key;

    if (value === null || value === undefined) {
      formData.append(formKey, '');
    } else if (Array.isArray(value)) {
      // Chuyển tất cả mảng thành JSON string
      formData.append(formKey, JSON.stringify(value));
    } else if (typeof value === 'object' && !(value instanceof File)) {
      toFormData(value, formKey, formData);
    } else {
      formData.append(formKey, value as string);
    }
  }

  return formData;
}

export function isAllowExtensions(fileName: string, extensions: string[], mimeType?: string) {
  if (!fileName) return true;
  const allowedLower = extensions.map((ext) => ext.toLowerCase());
  const parts = fileName.split('.');
  if (parts.length >= 2) {
    return allowedLower.includes(parts.pop()!.toLowerCase());
  }
  // Không có extension → fallback check MIME type (ví dụ: clipboard image)
  if (mimeType) {
    const ext = mimeType.split('/').pop()?.toLowerCase();
    return ext ? allowedLower.includes(ext === 'jpeg' ? 'jpg' : ext) || allowedLower.includes(ext) : false;
  }
  return false;
}

export function dequal(a: any, b: any): boolean {
  if (a === b) return true;
  if (a == null || b == null) return a === b;
  if (a instanceof Date && b instanceof Date) return a.getTime() === b.getTime();
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((v, i) => dequal(v, b[i]));
  }
  if (typeof a === 'object' && typeof b === 'object') {
    const ka = Object.keys(a), kb = Object.keys(b);
    return ka.length === kb.length && ka.every(k => dequal(a[k], b[k]));
  }
  return false;
}

/**
 * BE lưu MA_BENH_KT dạng chuỗi "code1;code2" (story-04, ≤12 mã). Dùng để hiển thị ra màn hình/bản in
 * — KHÔNG render raw chuỗi có dấu `;` vào ô 1-dòng thiết kế cho 1 mã (tràn khung, sai định dạng biểu
 * mẫu pháp lý BA-02..07/ba-18). `separator` mặc định ", " cho hiển thị đọc được.
 */
export function formatSecondaryIcdCodes(value: string | null | undefined, separator = ', '): string {
  if (!value) return '';
  return value.split(';').map((s) => s.trim()).filter(Boolean).join(separator);
}

/** icd_type trong bảng con `emr_record_icd` (story-05): 1=primary, 2=secondary, 3=differential. */
const ICD_TYPE_SECONDARY = 2;

/**
 * Hiển thị mã ICD "kèm theo" (MA_BENH_KT) cho print/màn hiển thị (story-29, gate của story-09
 * BE cutover). Ưu tiên đọc `icd_codes[]` (BE Get record, story-05 — đã sort icd_type ASC,
 * sort_order ASC) khi có, chỉ lấy icd_type=2 (secondary) — khớp đúng nội dung cột legacy
 * `secondary_icd10_code` trước đây (KHÔNG gồm primary/differential). Mảng rỗng/absent (record
 * cũ chưa backfill bảng con) → fallback parse chuỗi `secondary_icd10_code` như cũ.
 */
export function formatIcdCodesDisplay(
  icdCodes: Array<{ icd_code?: string; icd_type?: number }> | null | undefined,
  legacySecondaryIcd10Code: string | null | undefined,
  separator = ', ',
): string {
  if (Array.isArray(icdCodes) && icdCodes.length > 0) {
    return icdCodes
      .filter((c) => c?.icd_type === ICD_TYPE_SECONDARY)
      .map((c) => c.icd_code?.trim())
      .filter((v): v is string => !!v)
      .join(separator);
  }
  return formatSecondaryIcdCodes(legacySecondaryIcd10Code, separator);
}
