export interface GpsLocation {
  name: string;
  lat: number;
  lng: number;
  radius_meters: number;
}

export interface WifiNetwork {
  ssid: string;
  bssid: string;
}

export interface HrAttendanceConfigModel {
  id?: number;
  branch_id?: number | null;
  methods?: string[];           // ['gps','qr','wifi','excel']
  gps_config?: { locations: GpsLocation[] } | null;
  wifi_config?: { networks: WifiNetwork[] } | null;
  qr_config?: { refresh_minutes: number } | null;
  status?: number;
}

export interface HrAttendanceQrModel {
  id?: number;
  branch_id?: number | null;
  code?: string;
  expires_at?: number;
}

export interface CheckinRequest {
  method: 'gps' | 'qr' | 'wifi';
  lat?: number;
  lng?: number;
  qr_code?: string;
  wifi_ssid?: string;
  wifi_bssid?: string;
}
