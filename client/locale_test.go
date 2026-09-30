package main

import "testing"

func TestLangFromLANGID(t *testing.T) {
	cases := map[uint16]string{
		0x041B: langSK, // sk-SK
		0x001B: langSK, // neutral Slovak
		0x0409: langEN, // en-US
		0x0405: langEN, // cs-CZ
		0x0407: langEN, // de-DE
		0x0000: langEN,
	}
	for id, want := range cases {
		if got := langFromLANGID(id); got != want {
			t.Errorf("langFromLANGID(%#04x) = %q, want %q", id, got, want)
		}
	}
}

func TestLangFromTag(t *testing.T) {
	cases := map[string]string{
		"sk-SK":       langSK,
		"sk":          langSK,
		"SK":          langSK,
		"sk_SK.UTF-8": langSK,
		`"sk-SK"`:     langSK,
		"en-US":       langEN,
		"en":          langEN,
		"de":          langEN,
		"cs-CZ":       langEN,
		"sked":        langEN,
		"":            langEN,
		"C":           langEN,
	}
	for tag, want := range cases {
		if got := langFromTag(tag); got != want {
			t.Errorf("langFromTag(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestParseAppleLanguages(t *testing.T) {
	out := "(\n    \"sk-SK\",\n    \"en-SK\"\n)\n"
	if got := parseAppleLanguages(out); got != "sk-SK" {
		t.Fatalf("parseAppleLanguages = %q, want sk-SK", got)
	}
	if got := parseAppleLanguages("(\n    en,\n    sk\n)\n"); got != "en" {
		t.Fatalf("parseAppleLanguages unquoted = %q, want en", got)
	}
	if got := parseAppleLanguages(""); got != "" {
		t.Fatalf("parseAppleLanguages empty = %q, want \"\"", got)
	}
}

func TestLangFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if got := langFromEnv(env(map[string]string{"LANG": "sk_SK.UTF-8"})); got != langSK {
		t.Errorf("LANG=sk_SK → %q", got)
	}
	if got := langFromEnv(env(map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "sk_SK.UTF-8"})); got != langEN {
		t.Errorf("LC_ALL wins → %q", got)
	}
	if got := langFromEnv(env(map[string]string{"LANGUAGE": "sk:en"})); got != langSK {
		t.Errorf("LANGUAGE=sk:en → %q", got)
	}
	if got := langFromEnv(env(nil)); got != langEN {
		t.Errorf("empty env → %q", got)
	}
}

func TestTrIn(t *testing.T) {
	if got := trIn(langSK, "tray.quit"); got != "Ukončiť" {
		t.Errorf("sk tray.quit = %q", got)
	}
	if got := trIn(langEN, "tray.quit"); got != "Quit" {
		t.Errorf("en tray.quit = %q", got)
	}
	if got := trIn(langSK, "missing.key"); got != "missing.key" {
		t.Errorf("unknown key = %q", got)
	}
	for k, v := range goStrings {
		if v[0] == "" || v[1] == "" {
			t.Errorf("goStrings[%q] has an empty translation", k)
		}
	}
}
