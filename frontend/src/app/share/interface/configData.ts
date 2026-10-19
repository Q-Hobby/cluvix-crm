export interface ConfigData {
  dataDropDown: DataDropDownItem[];
  permission: PermissionItem[];
  options: OptionItem[];
  setting: SettingItem[];
  versionConfig: number;
}

export type DataDropDownItem = object;

export type PermissionItem = object;

export type OptionItem = object;

export type SettingItem = object;
