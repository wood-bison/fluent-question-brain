package httpapi

import "net/http"

// browse serves a read-only question browser. It is deliberately a single
// same-origin page with no build step and no external assets: the API already
// exposes everything it needs (/v1/quality for facets, /v1/catalog for the
// list, /v1/search for queries, /v1/questions/{key} for one card), and serving
// it from the API avoids both a second service and a CORS surface.
//
// It reads. It never writes. Promotion stays the CMS/command path.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, http.StatusOK, browsePage)
}

const browsePage = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Банк вопросов — Question Brain</title>
<style>
:root{
  color-scheme: light dark;
  --paper:#f6f7f9; --panel:#fff; --sunk:#eef1f5;
  --ink:#111820; --soft:#4a5866; --faint:#7a8794;
  --rule:#d9e0e7; --accent:#12657f; --accent-soft:#dcebf1;
  --warn:#8a5b00; --warn-soft:#fbefd8;
  --mono: ui-monospace, SFMono-Regular, Menlo, monospace;
}
@media (prefers-color-scheme: dark){
  :root{
    --paper:#0c1116; --panel:#141c23; --sunk:#101820;
    --ink:#e4ebf0; --soft:#a3b2be; --faint:#74858f;
    --rule:#26333d; --accent:#5fb6d1; --accent-soft:#12303c;
    --warn:#e0b466; --warn-soft:#342a12;
  }
}
*{box-sizing:border-box}
html,body{height:100%}
body{
  margin:0;background:var(--paper);color:var(--ink);
  font:15px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;
  -webkit-font-smoothing:antialiased;
}
header{
  display:flex;flex-wrap:wrap;gap:10px 16px;align-items:center;
  padding:12px 18px;border-bottom:1px solid var(--rule);background:var(--panel);
  position:sticky;top:0;z-index:5;
}
header h1{margin:0;font-size:15px;font-weight:600;letter-spacing:-.01em;white-space:nowrap}
header .count{font:500 12px var(--mono);color:var(--faint);white-space:nowrap}
.controls{display:flex;flex-wrap:wrap;gap:8px;flex:1 1 420px}
input[type=search],select{
  font:inherit;font-size:13px;padding:6px 9px;border:1px solid var(--rule);
  border-radius:7px;background:var(--panel);color:var(--ink);min-width:0;
}
input[type=search]{flex:1 1 200px}
select{flex:0 1 auto;max-width:170px}
input:focus-visible,select:focus-visible,button:focus-visible,li:focus-visible{
  outline:2px solid var(--accent);outline-offset:1px
}
button.reset{
  font:inherit;font-size:13px;padding:6px 11px;border:1px solid var(--rule);
  border-radius:7px;background:var(--sunk);color:var(--soft);cursor:pointer;
}
button.reset:hover{color:var(--ink)}

main{display:grid;grid-template-columns:minmax(300px,380px) 1fr;height:calc(100vh - 53px)}
@media (max-width:820px){main{grid-template-columns:1fr;height:auto}}

#list{border-right:1px solid var(--rule);overflow-y:auto;background:var(--panel)}
#list ol{list-style:none;margin:0;padding:0}
#list li{
  padding:11px 16px;border-bottom:1px solid var(--rule);cursor:pointer;
  display:flex;flex-direction:column;gap:5px;
}
#list li:hover{background:var(--sunk)}
#list li[aria-current=true]{background:var(--accent-soft);box-shadow:inset 3px 0 0 var(--accent)}
#list .p{font-size:13.5px;line-height:1.4}
#list .m{display:flex;flex-wrap:wrap;gap:5px}
.tag{
  font:500 10.5px var(--mono);text-transform:uppercase;letter-spacing:.06em;
  padding:2px 6px;border-radius:99px;background:var(--sunk);color:var(--faint);white-space:nowrap;
}
.tag.k{background:var(--accent-soft);color:var(--accent)}
.tag.c{background:var(--warn-soft);color:var(--warn)}

.pager{display:flex;gap:8px;align-items:center;justify-content:center;padding:12px;color:var(--faint);font:500 12px var(--mono)}
.pager button{
  font:inherit;padding:5px 11px;border:1px solid var(--rule);border-radius:7px;
  background:var(--panel);color:var(--ink);cursor:pointer
}
.pager button[disabled]{opacity:.4;cursor:default}

#detail{overflow-y:auto;padding:26px 30px 70px}
#detail .empty{color:var(--faint);margin-top:12vh;text-align:center}
#detail h2{margin:0 0 6px;font-size:21px;line-height:1.25;letter-spacing:-.01em}
#detail .keyline{font:500 12px var(--mono);color:var(--faint);margin-bottom:14px;word-break:break-all}
#detail .meta{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:20px}
#detail .lead{
  background:var(--accent-soft);border-left:4px solid var(--accent);
  padding:13px 16px;border-radius:0 8px 8px 0;margin-bottom:18px;
}
#detail .lead .lbl{font:500 10.5px var(--mono);text-transform:uppercase;letter-spacing:.1em;color:var(--accent);display:block;margin-bottom:5px}
section.sec{border-top:1px solid var(--rule);padding:15px 0}
section.sec h3{margin:0 0 7px;font-size:13px;letter-spacing:.02em;color:var(--soft)}
section.sec .txt{white-space:pre-wrap;font-size:14.5px;line-height:1.62;overflow-wrap:anywhere}
#detail .err{color:#a8271f;font:500 13px var(--mono)}
</style>
</head>
<body>
<header>
  <h1>Банк вопросов</h1>
  <span class="count" id="count">загрузка…</span>
  <div class="controls">
    <input type="search" id="q" placeholder="Поиск по смыслу и словам…" autocomplete="off" aria-label="Поиск">
    <select id="locale" aria-label="Язык"><option value="ru">RU</option><option value="en">EN</option></select>
    <select id="track" aria-label="Трек"><option value="">Все треки</option></select>
    <select id="level" aria-label="Уровень"><option value="">Все уровни</option></select>
    <select id="company" aria-label="Компания"><option value="">Все компании</option></select>
    <select id="topic" aria-label="Тема"><option value="">Все темы</option></select>
    <button class="reset" id="reset" type="button">Сброс</button>
  </div>
</header>

<main>
  <nav id="list" aria-label="Список вопросов"><ol id="items"></ol><div class="pager" id="pager"></div></nav>
  <article id="detail"><p class="empty">Выберите вопрос слева</p></article>
</main>

<script>
const WS = 'fluent-interview', PAGE = 50;
const $ = id => document.getElementById(id);
const el = (t, c, x) => { const n = document.createElement(t); if (c) n.className = c; if (x != null) n.textContent = x; return n; };
const state = { offset: 0, total: 0, items: [], current: null };
let seq = 0;

async function api(path) {
  const r = await fetch(path, { headers: { accept: 'application/json' } });
  if (!r.ok) throw new Error(path + ' → ' + r.status);
  return r.json();
}
const val = id => $(id).value.trim();

async function loadFacets() {
  const q = await api('/v1/quality?workspace=' + encodeURIComponent(WS));
  const fill = (id, rows, label) => {
    if (!Array.isArray(rows)) return;
    for (const row of rows) {
      const o = el('option', null, label(row));
      o.value = row.key;
      $(id).appendChild(o);
    }
  };
  fill('track', q.tracks, r => r.key + ' (' + r.count + ')');
  fill('level', q.levels, r => r.key + ' (' + r.count + ')');
  fill('company', q.companies, r => r.key + ' (' + r.count + ')');
  fill('topic', q.topics, r => r.key + ' (' + r.count + ')');
}

function tags(m, item) {
  const box = el('div', 'm');
  const add = (t, c) => { if (t) box.appendChild(el('span', 'tag' + (c ? ' ' + c : ''), t)); };
  add(item.stable_key, 'k');
  add(m.track); add(m.level);
  if (item.company && item.company !== 'unclassified') add(item.company, 'c');
  add(m.topic);
  return box;
}

function renderList() {
  const ol = $('items');
  ol.replaceChildren();
  if (!state.items.length) {
    const li = el('li'); li.appendChild(el('div', 'p', 'Ничего не найдено'));
    ol.appendChild(li); $('pager').replaceChildren(); return;
  }
  for (const it of state.items) {
    const li = el('li');
    li.tabIndex = 0;
    li.setAttribute('aria-current', String(it.stable_key === state.current));
    li.appendChild(el('div', 'p', it.prompt || it.stable_key));
    li.appendChild(tags(it.metadata || {}, it));
    const open = () => selectCard(it.stable_key);
    li.addEventListener('click', open);
    li.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); } });
    ol.appendChild(li);
  }
  const pager = $('pager');
  pager.replaceChildren();
  if (state.total > PAGE) {
    const prev = el('button', null, '←'), next = el('button', null, '→');
    prev.disabled = state.offset === 0;
    next.disabled = state.offset + PAGE >= state.total;
    prev.onclick = () => { state.offset = Math.max(0, state.offset - PAGE); load(); };
    next.onclick = () => { state.offset += PAGE; load(); };
    pager.append(prev, el('span', null, (state.offset + 1) + '–' + Math.min(state.offset + PAGE, state.total) + ' из ' + state.total), next);
  }
}

async function load() {
  const my = ++seq;
  const locale = val('locale'), text = val('q');
  try {
    if (text) {
      // Поиск не поддерживает трек и постраничность — показываем верх выдачи.
      const body = { query: text, locale, limit: PAGE };
      for (const k of ['topic_key', 'level', 'company']) {
        const src = k === 'topic_key' ? 'topic' : k;
        if (val(src)) body[k] = val(src);
      }
      const r = await fetch('/v1/search', {
        method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body)
      });
      if (!r.ok) throw new Error('search → ' + r.status);
      const d = await r.json();
      if (my !== seq) return;
      state.items = (d.results || []).map(x => ({
        stable_key: x.stable_key, prompt: x.prompt,
        metadata: { track: x.track, level: x.level, topic: x.topic_title }, company: x.company
      }));
      state.total = state.items.length; state.offset = 0;
      $('count').textContent = 'найдено ' + state.total + (state.total === PAGE ? '+' : '');
    } else {
      const p = new URLSearchParams({ workspace: WS, locale, limit: PAGE, offset: state.offset });
      for (const [id, key] of [['topic', 'topic_key'], ['track', 'track'], ['level', 'level'], ['company', 'company']]) {
        if (val(id)) p.set(key, val(id));
      }
      const d = await api('/v1/catalog?' + p);
      if (my !== seq) return;
      state.items = d.questions || [];
      state.total = d.total || 0;
      $('count').textContent = state.total + ' вопросов';
    }
    renderList();
  } catch (e) {
    if (my !== seq) return;
    $('count').textContent = 'ошибка';
    $('items').replaceChildren(Object.assign(el('li'), { innerHTML: '' }));
    $('items').firstChild.appendChild(el('div', 'p', String(e.message)));
  }
}

async function selectCard(key) {
  state.current = key;
  renderList();
  const d = $('detail');
  d.replaceChildren(el('p', 'empty', 'Загрузка…'));
  try {
    const c = await api('/v1/questions/' + encodeURIComponent(key) + '?workspace=' + encodeURIComponent(WS) + '&locale=' + encodeURIComponent(val('locale')));
    d.replaceChildren();
    d.appendChild(el('h2', null, c.prompt || key));
    d.appendChild(el('div', 'keyline', c.stable_key + ' · ревизия ' + String(c.revision_id || '').slice(0, 8) + ' · ' + (c.locale || '')));

    const meta = el('div', 'meta');
    for (const t of (c.topics || [])) meta.appendChild(el('span', 'tag' + (t.relation === 'primary' ? ' k' : ''), t.title));
    if (meta.childElementCount) d.appendChild(meta);

    if (c.short_answer) {
      const lead = el('div', 'lead');
      lead.appendChild(el('span', 'lbl', 'Короткий ответ'));
      lead.appendChild(el('div', 'txt', c.short_answer));
      d.appendChild(lead);
    }
    if (c.explanation) {
      const s = el('section', 'sec');
      s.appendChild(el('h3', null, 'Разбор'));
      s.appendChild(el('div', 'txt', c.explanation));
      d.appendChild(s);
    }
    for (const sec of ((c.body && c.body.sections) || [])) {
      if (!sec || !sec.body) continue;
      const s = el('section', 'sec');
      s.appendChild(el('h3', null, sec.title || '—'));
      s.appendChild(el('div', 'txt', sec.body));
      d.appendChild(s);
    }
  } catch (e) {
    d.replaceChildren(el('p', 'err', 'Не удалось загрузить карточку: ' + e.message));
  }
}

let t;
const debounced = () => { clearTimeout(t); t = setTimeout(() => { state.offset = 0; load(); }, 220); };
$('q').addEventListener('input', debounced);
for (const id of ['locale', 'track', 'level', 'company', 'topic']) {
  $(id).addEventListener('change', () => {
    state.offset = 0;
    load();
    if (id === 'locale' && state.current) selectCard(state.current);
  });
}
$('reset').addEventListener('click', () => {
  $('q').value = '';
  for (const id of ['track', 'level', 'company', 'topic']) $(id).value = '';
  state.offset = 0; load();
});

loadFacets().catch(() => {}).finally(load);
</script>
</body>
</html>`
