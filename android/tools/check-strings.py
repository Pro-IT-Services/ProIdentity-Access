"""Checks the Android string resources for ProIdentity Access.

1. Every R.string / R.plurals referenced in Kotlin exists in values/ and (unless
   translatable="false") in values-sk/.
2. values/ and values-sk/ have the same keys; plurals have the right quantities.
3. Format placeholders match between English and Slovak.
4. No hardcoded user-visible English literals remain in UI calls.
"""
import os, re, sys
import xml.etree.ElementTree as ET

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'app', 'src', 'main')
SRC = os.path.join(ROOT, 'java')

def load(path):
    strings, plurals, untranslatable = {}, {}, set()
    for el in ET.parse(path).getroot():
        name = el.get('name')
        if el.tag == 'string':
            strings[name] = el.text or ''
            if el.get('translatable') == 'false':
                untranslatable.add(name)
        elif el.tag == 'plurals':
            plurals[name] = {i.get('quantity'): i.text or '' for i in el}
    return strings, plurals, untranslatable

en_s, en_p, en_nt = load(os.path.join(ROOT, 'res/values/strings.xml'))
sk_s, sk_p, _ = load(os.path.join(ROOT, 'res/values-sk/strings.xml'))
errors = []

# 1. references
kt_files = [os.path.join(d, f) for d, _, fs in os.walk(SRC) for f in fs if f.endswith('.kt')]
refs_s, refs_p = set(), set()
for f in kt_files:
    t = open(f, encoding='utf-8').read()
    refs_s |= set(re.findall(r'R\.string\.(\w+)', t))
    refs_p |= set(re.findall(r'R\.plurals\.(\w+)', t))
manifest = open(os.path.join(ROOT, 'AndroidManifest.xml'), encoding='utf-8').read()
refs_s |= set(re.findall(r'@string/(\w+)', manifest))
for r in sorted(refs_s):
    if r not in en_s: errors.append(f'R.string.{r} missing in values/')
    if r not in sk_s and r not in en_nt: errors.append(f'R.string.{r} missing in values-sk/')
for r in sorted(refs_p):
    if r not in en_p: errors.append(f'R.plurals.{r} missing in values/')
    if r not in sk_p: errors.append(f'R.plurals.{r} missing in values-sk/')
unused = sorted((set(en_s) - refs_s) | (set(en_p) - refs_p))
if unused: errors.append(f'unused resources: {unused}')

# 2. key parity
for k in en_s:
    if k not in en_nt and k not in sk_s: errors.append(f'{k}: no Slovak translation')
for k in sk_s:
    if k not in en_s: errors.append(f'{k}: extra Slovak string')
    if k in en_nt: errors.append(f'{k}: translatable=false but translated')
for k, q in en_p.items():
    if set(q) != {'one', 'other'}: errors.append(f'{k}: English quantities {sorted(q)}')
    if k not in sk_p or set(sk_p[k]) != {'one', 'few', 'many', 'other'}:
        errors.append(f'{k}: Slovak needs one/few/many/other')

# 3. placeholders
ph = lambda s: sorted(re.findall(r'%\d+\$[sd]', s))
for k, v in en_s.items():
    if k in sk_s and ph(v) != ph(sk_s[k]): errors.append(f'{k}: placeholders differ {ph(v)} vs {ph(sk_s[k])}')
for k, q in en_p.items():
    for qq, v in sk_p.get(k, {}).items():
        if ph(v) != ph(q['other']): errors.append(f'{k}[{qq}]: placeholders differ')

# 4. leftover literals in UI calls
patterns = [
    r'\bText\(\s*"', r'\btitle\s*=\s*"', r'\bbody\s*=\s*"', r'\blabel\s*=\s*\{\s*Text\(\s*"',
    r'placeholder\s*=\s*\{\s*Text\(\s*"', r'contentDescription\s*=\s*"', r'actionLabel\s*=\s*"',
    r'primaryLabel\s*=\s*"', r'SectionLabel\(\s*"', r'InfoRow\(\s*"', r'showSnackbar\(\s*"',
    r'error\s*=\s*"', r'setError\(\s*"', r'Exception\(\s*"[A-Z]',
]
leftovers = []
for f in kt_files:
    for n, line in enumerate(open(f, encoding='utf-8'), 1):
        if any(re.search(p, line) for p in patterns):
            leftovers.append(f'{os.path.relpath(f, SRC)}:{n}: {line.strip()}')

print(f'strings: {len(en_s)} ({len(en_nt)} non-translatable), plurals: {len(en_p)}; '
      f'Slovak strings: {len(sk_s)}, plurals: {len(sk_p)}')
print(f'referenced: {len(refs_s)} strings, {len(refs_p)} plurals')
print('\nRemaining literals matched by UI patterns (review; intentional ones listed in the report):')
print('\n'.join(leftovers) or '  none')
if errors:
    print('\nERRORS:'); print('\n'.join('  ' + e for e in errors)); sys.exit(1)
print('\nOK')
