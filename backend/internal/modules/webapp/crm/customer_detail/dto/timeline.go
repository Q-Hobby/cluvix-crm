package dto

// TimelineEvent — 1 sự kiện hiển thị trong customer timeline.
type TimelineEvent struct {
	Title         string `json:"title"`
	Day           int64  `json:"day"`
	Content       string `json:"content"`
	IconDirection string `json:"iconDirection"`
	IconColor     string `json:"iconColor"`
}
