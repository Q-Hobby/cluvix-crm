// Khai báo dành cho update, xoá, view 1 dữ liệu cụ thể nào đó
export interface ApiResponseOneModel<T> {
  data: T;
  message: string;
}
