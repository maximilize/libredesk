package main

import (
	"testing"

	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
)

func TestOriginalTranslationMeta(t *testing.T) {
	meta := originalTranslationMeta(&translationOriginalReq{Language: "DE", Text: " Hallo Clara ", SentLanguage: "fr"})
	translations, ok := meta["translations"].(map[string]cmodels.MessageTranslation)
	if !ok {
		t.Fatalf("translations = %#v", meta["translations"])
	}
	de := translations["de"]
	if de.Text != "Hallo Clara" || de.Source != cmodels.TranslationSourceOriginal || de.From != "fr" || de.CreatedAt.IsZero() {
		t.Fatalf("de = %+v; want the trimmed original, marked original, sent as fr", de)
	}
	if meta["language"] != "fr" {
		t.Fatalf("language = %v, want fr", meta["language"])
	}

	if m := originalTranslationMeta(&translationOriginalReq{Language: "de", Text: "Hallo"}); m["language"] != nil {
		t.Fatalf("without a sent language, meta.language must stay unset: %v", m)
	}
	for _, o := range []*translationOriginalReq{nil, {Language: "", Text: "x"}, {Language: "de", Text: "  "}, {Language: "deutsch", Text: "x"}} {
		if m := originalTranslationMeta(o); m != nil {
			t.Errorf("originalTranslationMeta(%+v) = %v, want nil", o, m)
		}
	}
}
