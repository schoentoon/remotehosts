package remotehosts

import (
	"testing"
	"time"

	"github.com/caddyserver/caddy"
)

func TestHostsParse(t *testing.T) {
	tests := []struct {
		inputFileRules string
		shouldErr      bool
		expectedURLs   []string
		expectedReload time.Duration
	}{
		{
			`remotehosts
`,
			false,
			nil,
			time.Duration(0),
		},
		{
			`remotehosts {
	https://example.org/sample.txt
}`,
			false,
			[]string{"https://example.org/sample.txt"},
			time.Duration(0),
		},
		{
			`remotehosts {
	https://example.org/sample.txt
	reload 5m
}`,
			false,
			[]string{"https://example.org/sample.txt"},
			time.Minute * 5,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		h, err := hostsParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		} else if !test.shouldErr {
			if test.expectedReload != h.reload {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedReload, h.reload)
			}
			if test.expectedURLs == nil {
				continue
			}
			if len(h.URLs) != len(test.expectedURLs) {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedURLs, h.URLs)
			}
			for j, uri := range test.expectedURLs {
				if h.URLs[j].String() != uri {
					t.Fatalf("Test %d expected %v for %d th url, got %v", i, uri, j, h.URLs[j])
				}
			}
		}
	}
}
