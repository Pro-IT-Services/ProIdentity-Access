package main

import (
	"strings"
	"sync"
)

// UI language of the GUI process. The app is in Slovak when the user's
// primary OS UI language is Slovak and in English otherwise. Detection runs
// once per process (the OS language rarely changes while the app runs).

const (
	langEN = "en"
	langSK = "sk"
)

var (
	uiLangOnce sync.Once
	uiLang     = langEN
)

// currentLanguage returns "sk" or "en" for the logged-in user.
func currentLanguage() string {
	uiLangOnce.Do(func() {
		defer func() {
			// Never let language detection take the app down.
			if recover() != nil {
				uiLang = langEN
			}
		}()
		uiLang = detectSystemLanguage()
	})
	return uiLang
}

// SystemLanguage is bound to the frontend: "sk" when the OS UI language is
// Slovak, else "en".
func (a *App) SystemLanguage() string { return currentLanguage() }

// langFromLANGID maps a Windows LANGID to "sk"/"en". The primary language is
// the low 10 bits; LANG_SLOVAK is 0x1B (e.g. 0x041B = sk-SK).
func langFromLANGID(id uint16) string {
	if id&0x3FF == 0x1B {
		return langSK
	}
	return langEN
}

// langFromTag maps a BCP 47 / POSIX locale tag ("sk-SK", "sk_SK.UTF-8",
// "sk", "en-US", "de") to "sk"/"en" by its primary subtag.
func langFromTag(tag string) string {
	tag = strings.TrimSpace(strings.Trim(strings.TrimSpace(tag), `"'`))
	if i := strings.IndexAny(tag, "-_.@"); i >= 0 {
		tag = tag[:i]
	}
	if strings.EqualFold(tag, "sk") {
		return langSK
	}
	return langEN
}

// parseAppleLanguages returns the first entry of `defaults read -g
// AppleLanguages` output, which looks like:
//
//	(
//	    "sk-SK",
//	    "en-US"
//	)
func parseAppleLanguages(out string) string {
	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		s = strings.TrimSuffix(s, ",")
		s = strings.Trim(strings.TrimSpace(s), `"`)
		if s == "" || s == "(" || s == ")" {
			continue
		}
		return s
	}
	return ""
}

// langFromEnv follows the POSIX precedence LC_ALL > LC_MESSAGES > LANG, with
// LANGUAGE (a colon-separated preference list) first when set.
func langFromEnv(getenv func(string) string) string {
	if v := getenv("LANGUAGE"); v != "" {
		return langFromTag(strings.SplitN(v, ":", 2)[0])
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			return langFromTag(v)
		}
	}
	return langEN
}

// ── GUI-side strings (tray menu, a few errors from App methods) ──────────────

var goStrings = map[string][2]string{ // key → {en, sk}
	"tray.tooltip.connected":  {"ProIdentity VPN — Connected", "ProIdentity VPN — Pripojené"},
	"tray.tooltip.connecting": {"ProIdentity VPN — Connecting…", "ProIdentity VPN — Pripája sa…"},
	"tray.noTunnels":          {"No tunnels", "Žiadne tunely"},
	"tray.show":               {"Show", "Zobraziť"},
	"tray.showWindow":         {"Show window", "Zobraziť okno"},
	"tray.quit":               {"Quit", "Ukončiť"},
	"tray.quitApp":            {"Quit ProIdentity", "Ukončiť ProIdentity"},

	"err.serviceNotRunning":  {"the ProIdentity service is not running", "služba ProIdentity nie je spustená"},
	"err.noManagementServer": {"no management server is configured", "nie je nastavený žiadny server správy"},
	"err.nameConfigRequired": {"name and config are required", "zadajte názov aj konfiguráciu"},
	"err.profileNotFound":    {"profile not found", "profil sa nenašiel"},
}

// tr returns the GUI string for key in the user's language (the key itself
// if unknown, so a typo is visible rather than blank).
func tr(key string) string {
	return trIn(currentLanguage(), key)
}

func trIn(lang, key string) string {
	s, ok := goStrings[key]
	if !ok {
		return key
	}
	if lang == langSK {
		return s[1]
	}
	return s[0]
}
