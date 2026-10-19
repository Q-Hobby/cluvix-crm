import { Identifiable } from '@/core/interface/identifiable';

export interface VoiceBrandnameModel extends Identifiable {
  id?: number;
  name?: string;
  status?: number;
  provider_type?: number;
  config_ext?: any;
  config_other?: any;
  config_system?: any;
}

export interface ConfigExt {
  ext?: string;
  password?: string;
  userId?: number;
  phonenumber?: string;
  sipRealm?: string;
}
