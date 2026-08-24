#!/usr/bin/env python3
"""Generate cards from Avito interviewer PDFs (pdftotext -layout output)."""
import os, re, sys

V = os.path.expanduser('~/developer/fluent-question-vault/Question Cards')
TASK_HEAD = re.compile(r'^(.{3,80}?)\s*\((easy|medium|hard)\)\s*$')
QNUM = re.compile(r'^Вопрос №(\d+)\s+(.*)$')
RUB_HEAD = re.compile(r'^(Basic|Advanced|Expert|Начальный|Средний|Продвинутый|0 баллов|1 балл|3 балла)\s*:?\s*$')

def clean(s):
    s = s.replace('\u200b', '').replace('\u00ad', '')
    return re.sub(r'\s+', ' ', s).strip()

def parse_blocks(text):
    """Split into task blocks and theory question blocks."""
    lines = text.split('\n')
    blocks = []   # dict(kind, title, diff, lines)
    cur = None
    in_theory = False
    for ln in lines:
        s = clean(ln)
        if not s:
            if cur: cur['lines'].append('')
            continue
        if s in ('Рекомендуемые задачи',):
            continue
        if s.startswith('Теоретические вопросы'):
            in_theory = True
            continue
        m = TASK_HEAD.match(s)
        if m and not s.startswith('Теоретические'):
            cur = {'kind': 'task', 'title': m.group(1).strip(), 'diff': m.group(2), 'lines': []}
            blocks.append(cur)
            continue
        qm = QNUM.match(s)
        if qm:
            title_parts = [qm.group(2).strip()]
            cur = {'kind': 'theory', 'title': '', 'diff': None, 'lines': []}
            blocks.append(cur)
            # consume wrapped title lines until first bullet/rubric head
            j = len(blocks)  # placeholder to keep structure simple
            cur['_pending_title'] = title_parts
            continue
        if cur is not None and cur.get('_pending_title') is not None:
            if RUB_HEAD.match(s) or re.match(r'^[●○•\-–]', s):
                cur['title'] = ' '.join(cur.pop('_pending_title'))
                cur['lines'].append(ln.rstrip())
            else:
                cur['_pending_title'].append(s)
            continue
        if cur is not None:
            cur['lines'].append(ln.rstrip())
    return blocks

LEVEL = {'easy': 'Junior', 'medium': 'Middle', 'hard': 'Senior'}
DIFF = {'easy': 'EASY', 'medium': 'MEDIUM', 'hard': 'HARD'}

def rubric_from(lines):
    """Expectation bullets -> rubric levels; plain bullets -> single level."""
    rub, cur = [], None
    for raw in lines:
        s = clean(raw)
        if not s: continue
        hm = RUB_HEAD.match(s)
        if hm:
            cur = {'label': hm.group(1), 'text': []}
            rub.append(cur)
            continue
        bullet = re.sub(r'^[●○•\-–]\s*', '', s)
        if cur is not None:
            cur['text'].append(bullet)
        else:
            cur = {'label': 'Ожидания', 'text': [bullet]}
            rub.append(cur)
    return [{'label': r['label'], 'text': '\n'.join(r['text'])} for r in rub]

def emit(oz_id, track, topic, group, level, diff, company, q_en, q_ru, task, rubric, solution=None, scope='Avito', with_task=True):
    meta = [f'ID: {oz_id}', f'Track: {track}', f'Topic: {topic}', f'Scope: {scope}',
            'Lang:', 'Priority: common', f'Group: {group}', f'Level: {level}',
            f'Company: {company}']
    if diff: meta.append(f'Difficulty: {diff}')
    meta.append(f'Question: {q_en}')
    parts = [f'# {oz_id} — {clean(q_ru)[:80]}', '\n'.join(meta),
             '## Question (RU)', '', q_ru, '']
    if with_task:
        parts += ['## Task', '', task or q_ru, '']
    if solution:
        parts += ['## Solution', '', solution.strip(), '']
    if rubric:
        rl = []
        for r in rubric:
            first, _, rest = r['text'].partition('\n')
            rl.append(f"- {r['label']}: {first}")
            if rest: rl.append(rest)
        parts += ['## Рубрика', '', '\n'.join(rl), '']
    fname = f"{oz_id} — {re.sub(r'[\\\\/:*?\"<>|]+', ' ', clean(q_ru))[:70].strip()}.md"
    open(os.path.join(V, fname), 'w', encoding='utf-8').write('\n'.join(parts))

def run(cfg):
    text = open(cfg['txt'], encoding='utf-8').read()
    # cut boilerplate before first task/theory
    blocks = parse_blocks(text)
    n_t = n_th = 0
    for b in blocks:
        idx = cfg['next_id']()
        rubric = rubric_from(b['lines'])
        if b['kind'] == 'task':
            n_t += 1
            level = LEVEL.get(b['diff'], '')
            diff = DIFF.get(b['diff'], '')
            emit(idx, cfg['track'], cfg['topic'], 'Practical Tasks', level, diff,
                 cfg['company'], cfg['en'](b), b['title'], '\n'.join(b['lines']).strip(), rubric)
        else:
            n_th += 1
            emit(idx, cfg['track'], cfg['topic'], 'Common Questions', cfg.get('theory_level',''),
                 None, cfg['company'], cfg['en'](b), b['title'], None, rubric)
    print(f"{cfg['name']}: tasks={n_t} theory={n_th}")

if __name__ == '__main__':
    print('import as module')
