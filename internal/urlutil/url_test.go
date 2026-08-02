package urlutil

import "testing"

func TestNormalizeAndValidate(t *testing.T) {
	got := Normalize(" https://youtu.be/abc\n")
	if got != "https://youtu.be/abc" {
		t.Fatalf("Normalize() = %q", got)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestIsPlaylist(t *testing.T) {
	if !IsPlaylist("https://youtube.com/watch?v=12345678901&list=PL1") {
		t.Fatal("expected playlist URL")
	}
	if IsPlaylist("https://youtube.com/watch?v=12345678901&listing=no") {
		t.Fatal("listing must not count as list")
	}
}

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://youtube.com/watch?v=abcdefghijk": "abcdefghijk",
		"https://youtu.be/abcdefghijk?t=2":        "abcdefghijk",
		"https://vimeo.com/12345":                 "vimeo_12345",
	}
	for input, want := range cases {
		if got := VideoID(input); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", input, got, want)
		}
	}
}
