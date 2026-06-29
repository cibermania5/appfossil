package version

import "testing"

func TestFullWithoutMetadata(t *testing.T) {
	Version = "v0.2.0"
	Commit = ""
	Date = ""
	if got := Full(); got != "v0.2.0" {
		t.Fatalf("Full() = %q, want v0.2.0", got)
	}
}

func TestFullWithMetadata(t *testing.T) {
	Version = "v0.2.0"
	Commit = "abc1234"
	Date = "2026-06-29T12:00:00Z"
	want := "v0.2.0 (abc1234, 2026-06-29T12:00:00Z)"
	if got := Full(); got != want {
		t.Fatalf("Full() = %q, want %q", got, want)
	}
}

func TestLine(t *testing.T) {
	Version = "v0.2.0"
	Commit = "abc1234"
	Date = ""
	got := Line()
	if got != "appfossil v0.2.0 (abc1234)" {
		t.Fatalf("Line() = %q", got)
	}
}
