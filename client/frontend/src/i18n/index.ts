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
import { systemLanguage } from '../wailsbridge'

export type Lang = 'en' | 'sk'

type StringKey = { [K in keyof Dict]: Dict[K] extends string ? K : never }[keyof Dict]
type PluralKey = { [K in keyof Dict]: Dict[K] extends string ? never : K }[keyof Dict]
export type Params = Record<string, string | number>

const DICTS: Record<Lang, Record<keyof Dict, string | PluralForms>> = { en, sk }

export const useLanguage = create<{ lang: Lang; setLang: (l: Lang) => void }>((set) => ({
  lang: 'en',
  setLang: (lang) => {
    try { document.documentElement.lang = lang } catch { /* no DOM */ }
    set({ lang })
  },
}))

export const getLang = (): Lang => useLanguage.getState().lang

/** "sk" for a Slovak primary subtag ("sk", "sk-SK", "sk_SK.UTF-8"), else "en". */
export function normalizeLang(tag: string | null | undefined): Lang {
  return /^sk(?:$|[-_.@])/i.test((tag ?? '').trim()) ? 'sk' : 'en'
}

/** CLDR-style plural category. Slovak: 1 → one, 2–4 → few, 0 or 5+ → other. */
export function pluralCategory(lang: Lang, n: number): keyof PluralForms {
  const i = Math.abs(n)
  if (lang === 'sk') {
    if (!Number.isInteger(i)) return 'other'
    if (i === 1) return 'one'
    if (i >= 2 && i <= 4) return 'few'
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

/** Number with the language's decimal separator ("1.5" → "1,5" in Slovak). */
export function fmtDecimal(n: number, digits: number): string {
  const s = n.toFixed(digits)
  return getLang() === 'sk' ? s.replace('.', ',') : s
}

const DEV_OVERRIDE_KEY = 'proidentity:lang'

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

/**
 * Pick the UI language once at startup: the Go side's view of the OS UI
 * language, else the webview's navigator.language. Never throws.
 */
export async function initLanguage(): Promise<Lang> {
  let lang: Lang = 'en'
  const forced = devOverride()
  if (forced) {
    lang = forced
  } else {
    let fromGo = ''
    try {
      fromGo = await Promise.race([
        systemLanguage(),
        new Promise<string>(resolve => setTimeout(() => resolve(''), 1500)),
      ])
    } catch { /* fall through */ }
    lang = fromGo ? normalizeLang(fromGo) : normalizeLang(typeof navigator !== 'undefined' ? navigator.language : '')
  }
  useLanguage.getState().setLang(lang)
  return lang
}
