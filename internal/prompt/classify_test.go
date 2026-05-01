package prompt_test

import (
	"testing"

	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/prompt"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	policy := config.Builtins().FileReferences

	tests := []struct {
		name         string
		tok          prompt.Token
		wantKind     prompt.Kind
		wantExplicit bool
	}{
		{"image by ext", prompt.Token{RefPath: "/x/y.png"}, prompt.KindImage, false},
		{"jpg alt", prompt.Token{RefPath: "/x/y.JPG"}, prompt.KindImage, false},
		{"text by ext", prompt.Token{RefPath: "/x/y.md"}, prompt.KindText, false},
		{"unknown with error strategy", prompt.Token{RefPath: "/x/y.pdf"}, prompt.KindUnknown, false},
		{"override text", prompt.Token{RefPath: "/x/y.dat", KindOverride: prompt.KindText}, prompt.KindText, true},
		{"override image", prompt.Token{RefPath: "/x/y.raw", KindOverride: prompt.KindImage}, prompt.KindImage, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKind, gotExplicit := prompt.Classify(tt.tok, policy)
			if gotKind != tt.wantKind {
				t.Errorf("kind = %v; want %v", gotKind, tt.wantKind)
			}
			if gotExplicit != tt.wantExplicit {
				t.Errorf("explicit = %t; want %t", gotExplicit, tt.wantExplicit)
			}
		})
	}
}

func TestClassify_UnknownStrategies(t *testing.T) {
	t.Parallel()
	for _, strat := range []config.UnknownKind{config.UnknownText, config.UnknownImage} {
		t.Run(string(strat), func(t *testing.T) {
			t.Parallel()
			policy := config.Builtins().FileReferences
			policy.UnknownStrategy = strat
			got, _ := prompt.Classify(prompt.Token{RefPath: "/x/y.unknownext"}, policy)
			want := prompt.KindText
			if strat == config.UnknownImage {
				want = prompt.KindImage
			}
			if got != want {
				t.Errorf("strategy=%s: got %v want %v", strat, got, want)
			}
		})
	}
}
