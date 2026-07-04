package main

import (
	"errors"
	"testing"

	"ok-folio/internal/derivatives"
)

func TestParseWidths(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []int
	}{
		{name: "single", input: "400", want: []int{400}},
		{name: "multiple", input: "400,700", want: []int{400, 700}},
		{name: "trims and skips blanks", input: " 400 , ,700 ", want: []int{400, 700}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWidths(tc.input)
			if err != nil {
				t.Fatalf("parseWidths(%q) returned error: %v", tc.input, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseWidths(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("parseWidths(%q) = %v, want %v", tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestParseWidthsRejectsInvalid(t *testing.T) {
	for _, input := range []string{"abc", "0", "-5", "400,x"} {
		if _, err := parseWidths(input); err == nil {
			t.Fatalf("parseWidths(%q) expected error, got nil", input)
		}
	}
}

func TestParseWidthsRequiresAtLeastOne(t *testing.T) {
	if _, err := parseWidths("  , "); !errors.Is(err, derivatives.ErrNoWidths) {
		t.Fatalf("parseWidths blank input error = %v, want %v", err, derivatives.ErrNoWidths)
	}
}
