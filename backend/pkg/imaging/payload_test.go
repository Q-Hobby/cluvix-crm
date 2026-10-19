package imaging

import (
	"errors"
	"testing"
)

// DeleteOldIfChanged chỉ xoá key nằm dưới prefix của handler: giá trị cũ trong DB có thể là key client
// từng nhét qua payload (logo thương hiệu, file đính kèm riêng tư `private/...`).
func TestDeleteOldIfChanged_Prefix(t *testing.T) {
	st := newCleanupStore(t)
	for _, k := range []string{
		"user-profile/5/avatar/me.webp", "user-profile/6/avatar/other.webp",
		"business_profile/logo.webp", "private/customer/1/att.pdf", "branch/b.webp",
	} {
		putFile(t, st, k)
	}
	const pfx = "user-profile/5/avatar/"

	// Ngoài prefix ⇒ giữ nguyên (logo, avatar user khác, file riêng tư, prefix na ná).
	DeleteOldIfChanged(st, "business_profile/logo.webp", "", pfx)
	DeleteOldIfChanged(st, "user-profile/6/avatar/other.webp", "", pfx)
	DeleteOldIfChanged(st, "private/customer/1/att.pdf", "", pfx)
	DeleteOldIfChanged(st, st.URL("business_profile/logo.webp"), "", pfx) // URL legacy ⇒ strip rồi vẫn phải trong prefix
	DeleteOldIfChanged(st, "branch/b.webp", "")                           // không truyền prefix ⇒ không xoá
	DeleteOldIfChanged(st, "branch/b.webp", "", "", "branch")             // prefix rỗng / thiếu "/" ⇒ vô hiệu
	for _, k := range []string{"business_profile/logo.webp", "user-profile/6/avatar/other.webp", "private/customer/1/att.pdf", "branch/b.webp"} {
		if !exists(st, k) {
			t.Fatalf("%s nằm ngoài prefix — KHÔNG được xoá", k)
		}
	}

	// Trong prefix + giá trị đổi ⇒ xoá (kể cả cũ lưu dạng URL tuyệt đối của chính kho).
	DeleteOldIfChanged(st, st.URL("user-profile/5/avatar/me.webp"), "user-profile/5/avatar/new.webp", pfx)
	if exists(st, "user-profile/5/avatar/me.webp") {
		t.Fatal("ảnh cũ trong prefix phải bị xoá khi đổi")
	}
	DeleteOldIfChanged(st, "branch/b.webp", "", "service/", "branch/")
	if exists(st, "branch/b.webp") {
		t.Fatal("khớp 1 trong các prefix ⇒ phải xoá")
	}
}

func TestKeyInPrefixes(t *testing.T) {
	cases := []struct {
		key  string
		pfx  []string
		want bool
	}{
		{"branch/a.webp", []string{"branch/"}, true},
		{"branchX/a.webp", []string{"branch/"}, false},
		{"branch/", []string{"branch/"}, false},
		{"branch/a.webp", []string{"branch"}, false},
		{"branch/a.webp", []string{""}, false},
		{"branch/a.webp", nil, false},
		{"private/branch/a.webp", []string{"branch/"}, false},
		{"customer/KH1/avatar/a.webp", []string{"customer/tmp_1/avatar/", "customer/KH1/avatar/"}, true},
		{"x/a.webp", []string{"../x/"}, false},
	}
	for _, c := range cases {
		if got := KeyInPrefixes(c.key, c.pfx); got != c.want {
			t.Errorf("KeyInPrefixes(%q, %v) = %v, want %v", c.key, c.pfx, got, c.want)
		}
	}
}

func TestResolvePayloadImage(t *testing.T) {
	st := newCleanupStore(t)
	const cur = "branch/cur.webp"

	if v, err := ResolvePayloadImage(st, cur, ""); err != nil || v != "" {
		t.Fatalf("rỗng ⇒ xoá ảnh, got %q %v", v, err)
	}
	if v, err := ResolvePayloadImage(st, cur, cur); err != nil || v != cur {
		t.Fatalf("gửi lại key hiện tại ⇒ giữ, got %q %v", v, err)
	}
	if v, err := ResolvePayloadImage(st, cur, st.URL(cur)); err != nil || v != cur {
		t.Fatalf("gửi lại URL GET đã trả ⇒ giữ dạng key, got %q %v", v, err)
	}
	if v, err := ResolvePayloadImage(st, st.URL(cur), cur); err != nil || v != cur {
		t.Fatalf("DB legacy URL của kho ⇒ khớp key, got %q %v", v, err)
	}
	for _, bad := range []string{"business_profile/logo.webp", "private/customer/1/a.pdf", st.URL("branch/other.webp"), "https://evil.example.com/x.webp"} {
		v, err := ResolvePayloadImage(st, cur, bad)
		if !errors.Is(err, ErrImageKeyNotAllowed) || v != cur {
			t.Fatalf("%q phải bị từ chối và giữ ảnh hiện tại, got %q %v", bad, v, err)
		}
	}
	if _, err := ResolvePayloadImage(st, "", "branch/x.webp"); !errors.Is(err, ErrImageKeyNotAllowed) {
		t.Fatal("chưa có ảnh mà gửi key ⇒ phải từ chối")
	}
}
