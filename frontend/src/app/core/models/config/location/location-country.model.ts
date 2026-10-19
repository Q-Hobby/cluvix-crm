import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface LocationCountryModel extends Identifiable {
  id?: number;
  CountryCode?: string;
  name?: string;
  CommonName?: string;
  FormalName?: string;
  Capital?: string;
  CurrencyCode?: string;
  CurrencyName?: string;
  TelephoneCode?: string;
  SortOrder?: number;
  status?: number;
  created_by?: number;
  updated_by?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
