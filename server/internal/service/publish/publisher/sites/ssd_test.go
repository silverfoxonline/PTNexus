package sites

import (
	"testing"

	"github.com/pt-nexus/server/internal/service/publish/publisher"
)

func TestSSDSeriesPackFromSeasonTitle(t *testing.T) {
	input := publisher.PublishInput{
		Title: "Example.Show.S01.1080p.BluRay.Remux-ADE",
		UploadData: map[string]any{
			"standardized_params": map[string]any{
				"type": "category.tv_series",
			},
		},
	}

	if !isSSDSeriesPack(input) {
		t.Fatal("expected season-only TV title to be treated as CMCT pack")
	}
}

func TestSSDSeriesPackDoesNotMatchSingleEpisode(t *testing.T) {
	input := publisher.PublishInput{
		Title: "Example.Show.S01E02.1080p.WEB-DL-ADE",
		UploadData: map[string]any{
			"standardized_params": map[string]any{
				"type": "category.tv_series",
			},
		},
	}

	if isSSDSeriesPack(input) {
		t.Fatal("single episode should not be treated as CMCT pack")
	}
}
