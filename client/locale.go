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
	langCS = "cs"
	langPL = "pl"
	langHU = "hu"
	langDE = "de"
	langIT = "it"
	langES = "es"
)

// supportedLang reports whether code is one of the shipped UI languages.
func supportedLang(code string) bool {
	switch code {
	case langEN, langSK, langCS, langPL, langHU, langDE, langIT, langES:
		return true
	}
	return false
}

var (
	uiLangOnce sync.Once
	uiLang     = langEN

	uiLangMu       sync.Mutex
	uiLangOverride string // set from the frontend when the user picks a language
)

// detectedLanguage returns the OS UI language (one of the supported codes),
// detected once per process.
func detectedLanguage() string {
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

// currentLanguage returns the language used for GUI-side strings (tray,
// errors): the user's explicit choice when set, otherwise the OS language.
func currentLanguage() string {
	uiLangMu.Lock()
	o := uiLangOverride
	uiLangMu.Unlock()
	if o != "" {
		return o
	}
	return detectedLanguage()
}

// SystemLanguage is bound to the frontend: the detected OS UI language (one of
// the supported codes, else "en"). It ignores the user override so the
// frontend can offer a "follow system" option.
func (a *App) SystemLanguage() string { return detectedLanguage() }

// SetUILanguage is bound to the frontend: override the GUI-side language, or ""
// to follow the OS again.
func (a *App) SetUILanguage(lang string) {
	uiLangMu.Lock()
	defer uiLangMu.Unlock()
	if supportedLang(lang) {
		uiLangOverride = lang
	} else {
		uiLangOverride = ""
	}
}

// langFromLANGID maps a Windows LANGID to a supported code by its primary
// language (the low 10 bits), else "en".
func langFromLANGID(id uint16) string {
	switch id & 0x3FF {
	case 0x1B:
		return langSK
	case 0x05:
		return langCS
	case 0x15:
		return langPL
	case 0x0E:
		return langHU
	case 0x07:
		return langDE
	case 0x10:
		return langIT
	case 0x0A:
		return langES
	}
	return langEN
}

// langFromTag maps a BCP 47 / POSIX locale tag ("sk-SK", "de_DE.UTF-8", "pl",
// "en-US") to a supported code by its primary subtag, else "en".
func langFromTag(tag string) string {
	tag = strings.TrimSpace(strings.Trim(strings.TrimSpace(tag), `"'`))
	if i := strings.IndexAny(tag, "-_.@"); i >= 0 {
		tag = tag[:i]
	}
	tag = strings.ToLower(tag)
	switch tag {
	case "sk":
		return langSK
	case "cs", "cz":
		return langCS
	case "pl":
		return langPL
	case "hu":
		return langHU
	case "de":
		return langDE
	case "it":
		return langIT
	case "es", "ca", "gl":
		return langES
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

var goStrings = map[string]map[string]string{ // key → {lang → text}
	"tray.tooltip.connected": {
		langEN: "ProIdentity VPN — Connected", langSK: "ProIdentity VPN — Pripojené",
		langCS: "ProIdentity VPN — Připojeno", langPL: "ProIdentity VPN — Połączono",
		langHU: "ProIdentity VPN — Csatlakoztatva", langDE: "ProIdentity VPN — Verbunden",
		langIT: "ProIdentity VPN — Connesso", langES: "ProIdentity VPN — Conectado",
	},
	"tray.tooltip.connecting": {
		langEN: "ProIdentity VPN — Connecting…", langSK: "ProIdentity VPN — Pripája sa…",
		langCS: "ProIdentity VPN — Připojování…", langPL: "ProIdentity VPN — Łączenie…",
		langHU: "ProIdentity VPN — Csatlakozás…", langDE: "ProIdentity VPN — Verbinden…",
		langIT: "ProIdentity VPN — Connessione…", langES: "ProIdentity VPN — Conectando…",
	},
	"tray.noTunnels": {
		langEN: "No tunnels", langSK: "Žiadne tunely", langCS: "Žádné tunely", langPL: "Brak tuneli",
		langHU: "Nincsenek alagutak", langDE: "Keine Tunnel", langIT: "Nessun tunnel", langES: "Sin túneles",
	},
	"tray.show": {
		langEN: "Show", langSK: "Zobraziť", langCS: "Zobrazit", langPL: "Pokaż",
		langHU: "Megjelenítés", langDE: "Anzeigen", langIT: "Mostra", langES: "Mostrar",
	},
	"tray.showWindow": {
		langEN: "Show window", langSK: "Zobraziť okno", langCS: "Zobrazit okno", langPL: "Pokaż okno",
		langHU: "Ablak megjelenítése", langDE: "Fenster anzeigen", langIT: "Mostra finestra", langES: "Mostrar ventana",
	},
	"tray.quit": {
		langEN: "Quit", langSK: "Ukončiť", langCS: "Ukončit", langPL: "Zakończ",
		langHU: "Kilépés", langDE: "Beenden", langIT: "Esci", langES: "Salir",
	},
	"tray.quitApp": {
		langEN: "Quit ProIdentity", langSK: "Ukončiť ProIdentity", langCS: "Ukončit ProIdentity", langPL: "Zakończ ProIdentity",
		langHU: "Kilépés a ProIdentity-ből", langDE: "ProIdentity beenden", langIT: "Esci da ProIdentity", langES: "Salir de ProIdentity",
	},
	"err.serviceNotRunning": {
		langEN: "the ProIdentity service is not running", langSK: "služba ProIdentity nie je spustená",
		langCS: "služba ProIdentity není spuštěná", langPL: "usługa ProIdentity nie jest uruchomiona",
		langHU: "a ProIdentity szolgáltatás nem fut", langDE: "der ProIdentity-Dienst läuft nicht",
		langIT: "il servizio ProIdentity non è in esecuzione", langES: "el servicio ProIdentity no se está ejecutando",
	},
	"err.noManagementServer": {
		langEN: "no management server is configured", langSK: "nie je nastavený žiadny server správy",
		langCS: "není nastaven žádný server pro správu", langPL: "nie skonfigurowano serwera zarządzania",
		langHU: "nincs beállítva felügyeleti szerver", langDE: "es ist kein Verwaltungsserver konfiguriert",
		langIT: "nessun server di gestione configurato", langES: "no hay ningún servidor de gestión configurado",
	},
	"err.nameConfigRequired": {
		langEN: "name and config are required", langSK: "zadajte názov aj konfiguráciu",
		langCS: "zadejte název i konfiguraci", langPL: "wymagane są nazwa i konfiguracja",
		langHU: "a név és a konfiguráció megadása kötelező", langDE: "Name und Konfiguration sind erforderlich",
		langIT: "nome e configurazione sono obbligatori", langES: "se requieren el nombre y la configuración",
	},
	"err.profileNotFound": {
		langEN: "profile not found", langSK: "profil sa nenašiel", langCS: "profil nebyl nalezen", langPL: "nie znaleziono profilu",
		langHU: "a profil nem található", langDE: "Profil nicht gefunden", langIT: "profilo non trovato", langES: "perfil no encontrado",
	},
}

// tr returns the GUI string for key in the user's language (the key itself
// if unknown, so a typo is visible rather than blank).
func tr(key string) string {
	return trIn(currentLanguage(), key)
}

func trIn(lang, key string) string {
	m, ok := goStrings[key]
	if !ok {
		return key
	}
	if s, ok := m[lang]; ok {
		return s
	}
	return m[langEN]
}
