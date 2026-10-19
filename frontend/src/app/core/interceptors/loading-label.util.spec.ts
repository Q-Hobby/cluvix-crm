import { resolveLoadingLabelKey, resolveLoadingLabelKeyFromKey } from './loading-label.util';

describe('resolveLoadingLabelKey', () => {
  it('import POST → importing (KHÔNG phải creating)', () => {
    // Bug thật: POST /hr/user/import-users rơi vào rule POST generic, loader ghi "Đang tạo mới".
    expect(resolveLoadingLabelKey('POST', '/api/hr/user/import-users')).toBe('common.label.importing');
    expect(resolveLoadingLabelKey('POST', '/api/config/service/import-service-from-excel')).toBe('common.label.importing');
    expect(resolveLoadingLabelKey('POST', '/api/crm/customer/import')).toBe('common.label.importing');
    expect(resolveLoadingLabelKey('POST', '/api/med/x/9/import-by-id')).toBe('common.label.importing');
  });

  it('import-stock-export vẫn là IMPORT dù tên có chữ export', () => {
    expect(resolveLoadingLabelKey('POST', '/api/med/stock/import-stock-export')).toBe('common.label.importing');
  });

  it('không bắt nhầm từ chứa "import" như /important', () => {
    expect(resolveLoadingLabelKey('POST', '/api/notes/important')).toBe('common.label.creating');
  });

  it('export: endpoint riêng và quy ước ?_export=1', () => {
    expect(resolveLoadingLabelKey('POST', '/api/x/export-data-excel')).toBe('common.label.exporting');
    expect(resolveLoadingLabelKey('GET', '/api/emr/1/export-fhir')).toBe('common.label.exporting');
    expect(resolveLoadingLabelKey('GET', '/api/hr/user?_export=1')).toBe('common.label.exporting');
    expect(resolveLoadingLabelKey('GET', '/api/hr/user?page=2&_export=1')).toBe('common.label.exporting');
  });

  it('switch-profile (đổi thương hiệu) → switching (KHÔNG phải creating)', () => {
    expect(resolveLoadingLabelKey('POST', '/api/config/user-profile/switch-profile')).toBe('common.label.switching');
  });

  it('giữ nguyên các rule cũ', () => {
    expect(resolveLoadingLabelKey('POST', '/api/auth/login')).toBe('auth.login.submitting');
    expect(resolveLoadingLabelKey('POST', '/api/auth/logout')).toBe('common.label.logging_out');
    expect(resolveLoadingLabelKey('POST', '/api/crm/customer')).toBe('common.label.creating');
    expect(resolveLoadingLabelKey('PUT', '/api/crm/customer/1')).toBe('common.label.updating');
    expect(resolveLoadingLabelKey('DELETE', '/api/crm/customer/1')).toBe('common.label.deleting');
    expect(resolveLoadingLabelKey('OPTIONS', '/api/x')).toBe('common.label.loading');
  });
});

describe('resolveLoadingLabelKeyFromKey', () => {
  it('suy từ key của showLoading (đường duy nhất cho export GET)', () => {
    expect(resolveLoadingLabelKeyFromKey('exportUser')).toBe('common.label.exporting');
    expect(resolveLoadingLabelKeyFromKey('export-bydoctor')).toBe('common.label.exporting');
    expect(resolveLoadingLabelKeyFromKey('userImport')).toBe('common.label.importing');
    expect(resolveLoadingLabelKeyFromKey('importCustomer')).toBe('common.label.importing');
  });

  it('key không liên quan → undefined, giữ nguyên hành vi cũ', () => {
    expect(resolveLoadingLabelKeyFromKey('saveCustomer')).toBeUndefined();
    expect(resolveLoadingLabelKeyFromKey('loadReport')).toBeUndefined();
  });
});
