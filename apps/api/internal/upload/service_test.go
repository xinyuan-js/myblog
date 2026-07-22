package upload

import "testing"

func TestAcceptedImageRequiresMatchingExtension(t *testing.T) {
	tests := []struct {
		mime, extension, format string
		ok                      bool
	}{
		{"image/jpeg", ".jpg", "jpeg", true}, {"image/jpeg", ".jpeg", "jpeg", true},
		{"image/png", ".png", "png", true}, {"image/webp", ".webp", "webp", true},
		{"image/gif", ".gif", "gif", true}, {"image/png", ".jpg", "", false}, {"image/svg+xml", ".svg", "", false},
	}
	for _, tt := range tests {
		format, _, _, ok := acceptedImage(tt.mime, tt.extension)
		if ok != tt.ok || format != tt.format {
			t.Errorf("acceptedImage(%q,%q) = %q,%v", tt.mime, tt.extension, format, ok)
		}
	}
}

func TestEscapeLikeEscapesWildcards(t *testing.T) {
	if actual := escapeLike(`50%_off\today`); actual != `50\%\_off\\today` {
		t.Fatalf("escapeLike = %q", actual)
	}
}
