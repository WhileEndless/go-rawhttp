package render

import "testing"

func TestVirtualRecoverableJSON(t *testing.T) {
	vc := DefaultVirtual()
	for _, src := range []string{
		`{"a":1,"b":[true,false,null],"c":{"d":"x, y","e":{}},"f":[]}`,
		`[]`,
		`{}`,
		`{"s":"has } and , and \" inside"}`,
		`[1,2,[3,[4,5]],{"k":"v"}]`,
	} {
		got := BeautifyVirtual([]byte(src), LangJSON, vc)
		back := string(StripVirtual(got, vc))
		if back != src {
			t.Errorf("JSON not recoverable:\n src=%q\n got=%q\nback=%q", src, got, back)
		}
		// Rendered form must contain real newlines for a non-trivial object.
		if len(src) > 5 {
			rendered := string(RenderVirtual(got, vc, "  "))
			if rendered == src {
				t.Errorf("expected indentation in rendered form for %q", src)
			}
		}
	}
}

func TestVirtualRecoverableCSS(t *testing.T) {
	vc := DefaultVirtual()
	for _, src := range []string{
		`a{color:red;margin:0}.b{padding:2px}`,
		`/* c { x } */ a{content:"; }"}`,
		`@media(max-width:600px){.x{display:none}}`,
	} {
		got := BeautifyVirtual([]byte(src), LangCSS, vc)
		back := string(StripVirtual(got, vc))
		if back != src {
			t.Errorf("CSS not recoverable:\n src=%q\nback=%q", src, back)
		}
	}
}

func TestVirtualUnsupportedLangUnchanged(t *testing.T) {
	vc := DefaultVirtual()
	src := []byte(`function f(){return 1}`)
	if string(BeautifyVirtual(src, LangJS, vc)) != string(src) {
		t.Error("unsupported language should be returned unchanged")
	}
}
