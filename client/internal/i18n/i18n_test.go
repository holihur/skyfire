package i18n

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Lang{
		"":        EN,
		"en":      EN,
		"en-US":   EN,
		"EN_us":   EN,
		"zh":      ZH,
		"zh-CN":   ZH,
		"zh_CN":   ZH,
		"zh-Hans": ZH,
		"C":       EN,
		"fr":      EN, // unsupported falls back to default
	}
	for in, want := range cases {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestT(t *testing.T) {
	Set(EN)
	if got := T("status.connected"); got != "Connected" {
		t.Errorf("en status.connected = %q", got)
	}
	Set(ZH)
	if got := T("status.connected"); got != "已连接" {
		t.Errorf("zh status.connected = %q", got)
	}
	if got := T("tray.trafficValue", "1 B", "2 B"); got != "↓ 1 B   ↑ 2 B" {
		t.Errorf("formatted = %q", got)
	}
	if got := T("does.not.exist"); got != "does.not.exist" {
		t.Errorf("missing key = %q, want the key echoed", got)
	}
	Set(EN)
}

func TestCatalogParity(t *testing.T) {
	for _, l := range Supported() {
		for k := range catalog[EN] {
			if catalog[l][k] == "" {
				t.Errorf("language %s missing key %q", l, k)
			}
		}
	}
	for k := range catalog[ZH] {
		if catalog[EN][k] == "" {
			t.Errorf("english catalogue missing key %q", k)
		}
	}
}
