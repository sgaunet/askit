package prompt_test

import (
	"testing"

	"github.com/sgaunet/askit/internal/prompt"
)

func TestTokenize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantTokens []prompt.Token // partial: only check fields we set
	}{
		{
			name:      "pure prose",
			input:     "Please extract the text",
			wantCount: 1,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenText, Text: "Please extract the text"},
			},
		},
		{
			name:      "single @path",
			input:     "ocr @./scan.png",
			wantCount: 2,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenText, Text: "ocr "},
				{Kind: prompt.TokenFileRef, RefPath: "./scan.png"},
			},
		},
		{
			name:      "email-like not a ref",
			input:     "ping foo@bar.com now",
			wantCount: 1,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenText, Text: "ping foo@bar.com now"},
			},
		},
		{
			name:      "escaped @",
			input:     `contact me at \@example`,
			wantCount: 1,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenText, Text: "contact me at @example"},
			},
		},
		{
			name:      "image:kind suffix",
			input:     "@blob.dat:image",
			wantCount: 1,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenFileRef, RefPath: "blob.dat", KindOverride: prompt.KindImage},
			},
		},
		{
			name:      "text:kind suffix",
			input:     "@blob.dat:text",
			wantCount: 1,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenFileRef, RefPath: "blob.dat", KindOverride: prompt.KindText},
			},
		},
		{
			name:      "backtick-quoted path with spaces",
			input:     "ocr @`my scan.png` now",
			wantCount: 3,
			wantTokens: []prompt.Token{
				{Kind: prompt.TokenText, Text: "ocr "},
				{Kind: prompt.TokenFileRef, RefPath: "my scan.png"},
				{Kind: prompt.TokenText, Text: " now"},
			},
		},
		{
			name:      "multiple refs",
			input:     "a @./one.png and @./two.md",
			wantCount: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := prompt.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("token count = %d; want %d\ngot: %+v", len(got), tt.wantCount, got)
			}
			for i, want := range tt.wantTokens {
				if got[i].Kind != want.Kind {
					t.Errorf("tok[%d].Kind = %d; want %d", i, got[i].Kind, want.Kind)
				}
				if want.Text != "" && got[i].Text != want.Text {
					t.Errorf("tok[%d].Text = %q; want %q", i, got[i].Text, want.Text)
				}
				if want.RefPath != "" && got[i].RefPath != want.RefPath {
					t.Errorf("tok[%d].RefPath = %q; want %q", i, got[i].RefPath, want.RefPath)
				}
				if want.KindOverride != prompt.KindUnknown && got[i].KindOverride != want.KindOverride {
					t.Errorf("tok[%d].KindOverride = %d; want %d", i, got[i].KindOverride, want.KindOverride)
				}
			}
		})
	}
}

func TestTokenize_UnterminatedBacktickErrors(t *testing.T) {
	t.Parallel()
	_, err := prompt.Tokenize("ocr @`open and never close")
	if err == nil {
		t.Fatal("want error on unterminated backtick")
	}
}
