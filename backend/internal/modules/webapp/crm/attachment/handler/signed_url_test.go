package handler

import "testing"

// inlineViewable — chỉ ảnh raster, PDF, audio/video TRONG whitelist được inline; còn lại attachment.
func TestInlineViewable(t *testing.T) {
	cases := map[string]bool{
		"image/png": true, "image/webp": true, "application/pdf": true,
		"video/mp4": true, "video/webm": true, "audio/mpeg": true, "audio/ogg": true,
		"image/svg+xml": false, "text/html": false, "text/plain": false, "application/zip": false,
		"application/octet-stream": false, "video/x-unknown": false, "audio/x-evil": false, "": false,
	}
	for ct, want := range cases {
		if got := inlineViewable(ct); got != want {
			t.Errorf("inlineViewable(%q)=%v, muốn %v", ct, got, want)
		}
	}
}
