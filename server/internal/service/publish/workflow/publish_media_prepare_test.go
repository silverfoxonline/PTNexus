package workflow

import "testing"

func TestBuildPublishScopedMediaRootsDoesNotScanBroadSaveRoot(t *testing.T) {
	roots := buildPublishScopedMediaRoots("/downloads/keep", "Show.S01.1080p-ADE", "Show.S01.1080p-ADE")

	if len(roots) != 1 {
		t.Fatalf("roots len = %d, want 1: %#v", len(roots), roots)
	}
	if roots[0] != "/downloads/keep/Show.S01.1080p-ADE" {
		t.Fatalf("roots[0] = %q", roots[0])
	}
}

func TestBuildPublishScopedMediaRootsAllowsSavePathWhenItIsTorrentRoot(t *testing.T) {
	roots := buildPublishScopedMediaRoots("/downloads/keep/Show.S01.1080p-ADE", "Show.S01.1080p-ADE", "")

	if len(roots) != 2 {
		t.Fatalf("roots len = %d, want 2: %#v", len(roots), roots)
	}
	if roots[1] != "/downloads/keep/Show.S01.1080p-ADE" {
		t.Fatalf("save path root was not retained: %#v", roots)
	}
}
