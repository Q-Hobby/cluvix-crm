package handler

import (
	"bytes"
	"encoding/json"

	"erpCluvix/pkg/define"
)

// layoutConfig — hình dạng `dashboard_layout.config` (hợp đồng với FE `DashboardLayoutConfig`).
type layoutConfig struct {
	Version int          `json:"version"`
	Items   []layoutItem `json:"items"`
}

type layoutItem struct {
	Key     string `json:"key"`
	Zone    string `json:"zone"`
	Visible bool   `json:"visible"`
	Size    string `json:"size"`
}

// parseConfig validate hình dạng + allowlist key + trần, trả bản chuẩn hoá (re-marshal) để lưu. Trả i18n key
// khi lỗi. Thứ tự mảng = thứ tự hiển thị trong vùng.
func parseConfig(raw json.RawMessage) ([]byte, string) {
	if len(raw) > define.CrmDashboardLayoutMaxBytes {
		return nil, "crm_dashboard_layout_invalid"
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, "crm_dashboard_layout_invalid"
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var cfg layoutConfig
	if err := dec.Decode(&cfg); err != nil || dec.More() {
		return nil, "crm_dashboard_layout_invalid"
	}
	if cfg.Version != define.CrmDashboardLayoutVersion || cfg.Items == nil || len(cfg.Items) > define.CrmDashboardLayoutMaxItems {
		return nil, "crm_dashboard_layout_invalid"
	}
	seen := map[string]bool{}
	for _, it := range cfg.Items {
		if !define.CrmDashboardWidgetKeys[it.Key] || seen[it.Key] {
			return nil, "crm_dashboard_layout_invalid"
		}
		seen[it.Key] = true
		switch it.Zone {
		case define.CrmDashboardZoneTop, define.CrmDashboardZoneMain, define.CrmDashboardZoneSide:
		default:
			return nil, "crm_dashboard_layout_invalid"
		}
		if it.Size != define.CrmDashboardSizeHalf && it.Size != define.CrmDashboardSizeFull {
			return nil, "crm_dashboard_layout_invalid"
		}
	}
	norm, err := json.Marshal(cfg)
	if err != nil {
		return nil, "crm_dashboard_layout_invalid"
	}
	return norm, ""
}
