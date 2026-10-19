import { Identifiable } from '@/core/interface/identifiable';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { UserProfileModel } from '@/core/models/user-profile.model';

export interface LocationProvinceModel extends Identifiable {
  id?: number;
  codeProvince?: string;
  name?: string;
  Type?: string;
  status?: number;
  SortOrder?: number;
  updated_at?: any;
  created_at?: any;
  created_by?: any;
  TelephoneCode?: any;
  CountryId?: number;
  CountryCode?: string;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
  country?: LocationCountryModel;
}
