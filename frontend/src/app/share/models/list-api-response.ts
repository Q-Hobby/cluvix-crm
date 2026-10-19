export interface ListApiResponse<T> {
  items?: T[];
  _meta?: PaginationInfo;
}

export interface PaginationInfo {
  totalCount?: number;
  pageCount?: number;
  currentPage?: number;
  perPage?: number;
}
