package persist

import (
	"reflect"
	"testing"
)

func TestNormalizeSeedTagsForReviewPreservesPublishTags(t *testing.T) {
	got := NormalizeSeedTagsForReview([]string{"官方", "中字", "tag.合集", "中字", "官字组"})
	want := []string{"tag.中字", "tag.合集"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeSeedTagsForReview() = %#v, want %#v", got, want)
	}
}

func TestNormalizeSeedTagsForReviewParsesJSON(t *testing.T) {
	got := NormalizeSeedTagsForReview(`["中字","国语"]`)
	want := []string{"tag.中字", "tag.国语"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeSeedTagsForReview() = %#v, want %#v", got, want)
	}
}
