/**
 * Tiny dependency-free i18n. English is the default; Slovak is used when the
 * OS UI language is Slovak (detected by the Go side, see locale.go).
 *
 *   t('common.connect')                     → "Connect" / "Pripojiť"
 *   t('topbar.signOutAs', { username })     → "{username}" interpolation
 *   tp('time.daysAgo', 3)                   → plural form, {count} filled in
 *   tNode('conns.empty', { file: <span/> }) → string/ReactNode parts for JSX
 */
import type { ReactNode } from 'react'
import { create } from 'zustand'
import { en, type Dict, type PluralForms } from './en'
import { sk } from './sk'
import { cs } from './cs'
import { pl } from './pl'
import { hu } from './hu'
import { de } from './de'
import { it } from './it'
import { es } from './es'
import { systemLanguage, setUILanguage } from '../wailsbridge'

export type Lang = 'en' | 'sk' | 'cs' | 'pl' | 'hu' | 'de' | 'it' | 'es'

/** Supported languages in menu order, with their endonyms for the picker. */
export const LANG_NAMES: Record<Lang, string> = {
  en: 'English',
  sk: 'Slovenčina',
  cs: 'Čeština',
  pl: 'Polski',
  hu: 'Magyar',
  de: 'Deutsch',
  it: 'Italiano',
  es: 'Español',
}
export const LANGS = Object.keys(LANG_NAMES) as Lang[]

type StringKey = { [K in keyof Dict]: Dict[K] extends string ? K : never }[keyof Dict]
type PluralKey = { [K in keyof Dict]: Dict[K] extends string ? never : K }[keyof Dict]
export type Params = Record<string, string | number>

const DICTS: Record<Lang, Record<keyof Dict, string | PluralForms>> = { en, sk, cs, pl, hu, de, it, es }

export const useLanguage = create<{ lang: Lang; setLang: (l: Lang) => void }>((set) => ({
  lang: 'en',
  setLang: (lang) => {
    try { document.documentElement.lang = lang } catch { /* no DOM */ }
    set({ lang })
  },
}))

export const getLang = (): Lang => useLanguage.getState().lang

/** Map an OS locale tag to a supported language by its primary subtag, else "en". */
export function normalizeLang(tag: string | null | undefined): Lang {
  const primary = (tag ?? '').trim().toLowerCase().split(/[-_.@]/)[0]
  const map: Record<string, Lang> = {
    en: 'en', sk: 'sk',
    cs: 'cs', cz: 'cs',          // Czech (cz is a common misspelling)
    pl: 'pl', hu: 'hu', de: 'de', it: 'it',
    es: 'es', ca: 'es', gl: 'es', // Spanish; map Catalan/Galician to Spanish
  }
  return map[primary] ?? 'en'
}

/**
 * CLDR-style plural category for the 3 plural keys.
 * Slovak/Czech: 1 → one, 2–4 → few, else → other.
 * Polish: 1 → one, 2–4 (but not 12–14) → few, else → other.
 * Everything else: 1 → one, else → other.
 */
export function pluralCategory(lang: Lang, n: number): keyof PluralForms {
  const i = Math.abs(n)
  if (lang === 'sk' || lang === 'cs') {
    if (!Number.isInteger(i)) return 'other'
    if (i === 1) return 'one'
    if (i >= 2 && i <= 4) return 'few'
    return 'other'
  }
  if (lang === 'pl') {
    if (!Number.isInteger(i)) return 'other'
    if (i === 1) return 'one'
    const t10 = i % 10, t100 = i % 100
    if (t10 >= 2 && t10 <= 4 && !(t100 >= 12 && t100 <= 14)) return 'few'
    return 'other'
  }
  return i === 1 ? 'one' : 'other'
}

function interpolate(s: string, params?: Params): string {
  if (!params) return s
  return s.replace(/\{(\w+)\}/g, (m, k: string) => (k in params ? String(params[k]) : m))
}

function lookup(key: keyof Dict): string | PluralForms {
  return DICTS[getLang()][key] ?? en[key]
}

/** Translate a plain string key. */
export function t(key: StringKey, params?: Params): string {
  const v = lookup(key)
  return interpolate(typeof v === 'string' ? v : v.other, params)
}

/** Translate a plural key for `count` ({count} is available in the text). */
export function tp(key: PluralKey, count: number, params?: Params): string {
  const v = lookup(key)
  if (typeof v === 'string') return interpolate(v, { count, ...params })
  const form = v[pluralCategory(getLang(), count)] ?? v.other
  return interpolate(form, { count, ...params })
}

/** Like t(), but placeholders may be React nodes (e.g. a styled span). */
export function tNode(key: StringKey, nodes: Record<string, ReactNode>): ReactNode[] {
  const s = t(key)
  const out: ReactNode[] = []
  let last = 0
  s.replace(/\{(\w+)\}/g, (m, k: string, offset: number) => {
    if (!(k in nodes)) return m
    if (offset > last) out.push(s.slice(last, offset))
    out.push(nodes[k])
    last = offset + m.length
    return m
  })
  if (last < s.length) out.push(s.slice(last))
  return out
}

/** Number with the language's decimal separator (comma for every language except English). */
export function fmtDecimal(n: number, digits: number): string {
  const s = n.toFixed(digits)
  return getLang() === 'en' ? s : s.replace('.', ',')
}

const DEV_OVERRIDE_KEY = 'proidentity:lang'
const PREF_KEY = 'proidentity:uiLangPref'

/** Dev-only override: `?lang=sk` in the URL or localStorage["proidentity:lang"]. */
function devOverride(): Lang | null {
  if (!(import.meta as any).env?.DEV) return null
  try {
    const q = new URLSearchParams(window.location.search).get('lang')
    if (q) {
      localStorage.setItem(DEV_OVERRIDE_KEY, q)
      return normalizeLang(q)
    }
    const saved = localStorage.getItem(DEV_OVERRIDE_KEY)
    return saved ? normalizeLang(saved) : null
  } catch {
    return null
  }
}

/** The user's language choice: a specific language, or 'system' to follow the OS. */
export type LangPreference = Lang | 'system'

/** Read the saved language choice; defaults to 'system' (follow the OS). */
export function getLangPreference(): LangPreference {
  try {
    const v = localStorage.getItem(PREF_KEY)
    if (v === 'system' || (v && (LANGS as string[]).includes(v))) return v as LangPreference
  } catch { /* ignore */ }
  return 'system'
}

/** Detect the OS language via the Go side, falling back to the webview locale. */
async function detectSystemLang(): Promise<Lang> {
  let fromGo = ''
  try {
    fromGo = await Promise.race([
      systemLanguage(),
      new Promise<string>(resolve => setTimeout(() => resolve(''), 1500)),
    ])
  } catch { /* fall through */ }
  return fromGo ? normalizeLang(fromGo) : normalizeLang(typeof navigator !== 'undefined' ? navigator.language : '')
}

async function resolvePreference(pref: LangPreference): Promise<Lang> {
  return pref === 'system' ? detectSystemLang() : pref
}

/** Apply a resolved language: tell the Go side (tray/errors) and update the store. */
function applyLang(lang: Lang, pref: LangPreference) {
  void setUILanguage(pref === 'system' ? '' : lang)
  useLanguage.getState().setLang(lang)
}

/** Change the user's language choice from Settings, persisting it. The app remounts. */
export async function setLangPreference(pref: LangPreference): Promise<Lang> {
  try { localStorage.setItem(PREF_KEY, pref) } catch { /* ignore */ }
  const lang = await resolvePreference(pref)
  applyLang(lang, pref)
  return lang
}

/**
 * Pick the UI language once at startup: a dev override, then the saved choice,
 * then the OS language (English if it isn't one of the supported languages).
 */
export async function initLanguage(): Promise<Lang> {
  const forced = devOverride()
  if (forced) {
    useLanguage.getState().setLang(forced)
    return forced
  }
  const pref = getLangPreference()
  const lang = await resolvePreference(pref)
  applyLang(lang, pref)
  return lang
}
