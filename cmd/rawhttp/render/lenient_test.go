package render

import (
	"strings"
	"testing"
)

func TestLenientBestEffort(t *testing.T) {
	cases := []struct {
		lang, src string
		wantNL    bool
	}{
		{LangJSON, `{"a":1,"b":[1,2,{"c":"x`, true}, // truncated -> indented
		{LangJS, `function f(){if(x){return 1;`, true},
		{LangJSON, `not json at all`, false}, // no structure -> unchanged, no crash
	}
	for _, c := range cases {
		out := string(Beautify([]byte(c.src), c.lang))
		if out == "" {
			t.Errorf("%s: empty output", c.lang)
		}
		if c.wantNL && !strings.Contains(out, "\n") {
			t.Errorf("%s: expected best-effort indentation, got %q", c.lang, out)
		}
	}
}

func TestLenientStringAware(t *testing.T) {
	// Structural chars inside a string must NOT trigger line breaks, and content
	// must be preserved.
	out := string(Beautify([]byte(`{"x":"a,{b}c`), LangJSON))
	if !strings.Contains(out, `"a,{b}c`) {
		t.Errorf("string content altered: %q", out)
	}
}
