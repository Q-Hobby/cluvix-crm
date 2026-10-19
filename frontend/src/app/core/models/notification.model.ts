import { Identifiable } from '@/core/interface/identifiable';

export interface NotificationModel extends Identifiable {
  id?: number;
  name?: string;
  content?: string;
  category_id?: number;
  filter_from?: number;
  filter_to?: number;
  createdAt?: number;

  categoryString?: string;
}
