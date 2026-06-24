package repair

import "testing"

func TestPixhostThumbToDirectURLPreservesFilename(t *testing.T) {
	thumbURL := "https://t2.pixhost.to/thumbs/8851/742623425_m73g1j.png"
	want := "https://img2.pixhost.to/images/8851/742623425_m73g1j.png"

	if got := PixhostThumbToDirectURL(thumbURL); got != want {
		t.Fatalf("PixhostThumbToDirectURL() = %q, want %q", got, want)
	}
}

func TestPixhostShowToDirectURLDoesNotGuessImageHost(t *testing.T) {
	showURL := "https://pixhost.to/show/8851/742623425_m73g1j.png"

	if got := PixhostShowToDirectURL(showURL); got != "" {
		t.Fatalf("PixhostShowToDirectURL() = %q, want empty without thumbnail host", got)
	}
}
