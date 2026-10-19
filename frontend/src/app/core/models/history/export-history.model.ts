import { Identifiable } from '@/core/interface/identifiable';

export interface ExportHistoryModel extends Identifiable {
  id?: number;
  positionExport?: string;
  quantityRecord?: number;
  ip_address?: string;
  userAgent?: string;
  createdBy?: number;
  createdAt?: number;
}
