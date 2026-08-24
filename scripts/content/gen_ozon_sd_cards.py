#!/usr/bin/env python3
"""Generate System Design cards from system_design_ozon 23.docx extraction."""
import json, os, re

from quality import require_clean_text, require_prompt

lines = open('/tmp/qb/ozon_sd.txt', encoding='utf-8').read().split('\n')
OUT = os.path.expanduser('~/developer/fluent-question-vault/Question Cards')
num_re = re.compile(r'^(\d{1,2})\.$')
GRADE_HEAD = re.compile(r'^(1[7-9]|2[01])\s*грейд\s*$')
MARKERS = {'Architecture (System Design)', 'System Design Tasks'}

EN = {
1:"Design a phone number classifier: country/region lookup optimized for very frequent calls",
2:"Design a scalability strategy for the Telephony Platform",
3:"Design a compute platform",
4:"Explain data replication in storage systems",
5:"What is Domain-Driven Design?",
6:"How do column-oriented databases work, and when do they fit?",
7:"Give an overview of non-relational (NoSQL) databases",
8:"How does distributed locking work?",
9:"Explain the CAP theorem",
10:"What cache invalidation approaches exist?",
11:"Which caching strategies do you know?",
12:"What is service discovery?",
13:"What message delivery guarantees exist?",
14:"What is idempotency and how do you implement it?",
15:"Which load balancing algorithms do you know?",
16:"What are sticky sessions and why are they needed?",
17:"Client-side vs server-side load balancing — what is the difference?",
18:"Design a Messenger (from 10k to 10M users): capacity estimate, components, data flow",
19:"Design Web Search for 1 trillion pages with sub-200ms latency",
20:"Design Instagram: 500M DAU, news feed fan-out on write vs on read",
21:"How do you guarantee writing an event to both the database and a queue (Transactional Outbox)?",
22:"Which two approaches exist to implement a Saga (orchestration vs choreography)?",
23:"What is consistent hashing and where is it applicable?",
24:"How is a distributed transaction implemented with two-phase commit (2PC)?",
25:"What is a Saga?",
26:"How do you choose the format of inter-service communication?",
27:"What is Event Sourcing?",
28:"What is Command Query Responsibility Segregation (CQRS), and when does it pay off?",
29:"Walk through a violation of 1–2 SOLID principles (interviewer's choice)",
30:"Differences between three-layer, hexagonal and onion architectures",
31:"Walk through 1–2 simple design patterns (singleton, strategy, adapter, …)",
32:"Define 1–2 SOLID principles in your own words (interviewer's choice)",
33:"Monolith vs service-oriented vs microservice architecture — what are the differences?",
}

def split_task(i_start, title):
    j = i_start + 1
    body = []
    while j < len(lines) and not num_re.match(lines[j].strip()):
        body.append(lines[j]); j += 1
    cond, rubric, cur = [], [], None
    seen_ans = False
    for ln in body:
        s = ln.strip()
        if s in MARKERS or not s:
            continue
        gm = GRADE_HEAD.match(s)
        if gm:
            cur = {'label': f'{gm.group(1)} грейд', 'text': []}
            rubric.append(cur); seen_ans = True
            continue
        if cur is not None:
            cur['text'].append(ln.rstrip())
        else:
            if s == 'Ответ':
                seen_ans = True; continue
            cond.append(ln.rstrip())
    return {
        'title': title,
        'condition': '\n'.join(l for l in cond if l.strip()).strip(),
        'rubric': [{'label': r['label'], 'text': '\n'.join(r['text']).strip()} for r in rubric],
        'graded': bool(rubric),
        'answer_plain': '' if (seen_ans or rubric) else '',
    }

def main():
    tasks = []
    i = 0
    while i < len(lines):
        m = num_re.match(lines[i].strip())
        if m and i + 1 < len(lines):
            t = split_task(i, lines[i+1].strip())
            t['no'] = int(m.group(1))
            tasks.append(t)
        i += 1
    assert len(tasks) == 33, len(tasks)
    written = 0
    for idx, t in enumerate(tasks):
        oz_id = f'OZ-{164 + idx}'
        ru_q = re.sub(r'\s+', ' ', t['title'])
        source = f'Ozon:{oz_id}'
        english_prompt = EN[int(oz_id.split('-')[1]) - 163]
        require_prompt(english_prompt, source=source, field='Question (EN)', title=ru_q, topic='System Design')
        require_prompt(ru_q, source=source, field='Question (RU)', title=ru_q, topic='System Design')
        require_clean_text(t['condition'], source=source, field='Task')
        level = ''
        lvmap = {'17':'Junior','18':'Middle','19':'Middle+','20':'Senior','21':'Senior'}
        grades = [r['label'].split()[0] for r in t['rubric']]
        lv = [lvmap[g] for g in grades if g in lvmap]
        level = lv[-1] if lv else ''
        rub_lines = []
        for r in t['rubric']:
            txt = r['text'] or '(ответ отсутствует в источнике)'
            first, _, rest = txt.partition('\n')
            rub_lines.append(f"- {r['label']}: {first}")
            if rest: rub_lines.append(rest)
        meta = [f'ID: {oz_id}', 'Track: Backend', 'Topic: System Design',
                'Scope: Ozon', 'Lang:', 'Priority: common', 'Group: System Design',
                f'Level: {level}', 'Company: Ozon', f'Question: {english_prompt}']
        parts = [f'# {oz_id} — {ru_q[:80]}', '\n'.join(meta),
                 '## Question (RU)', '', ru_q, '']
        if t['graded']:
            parts += ['## Task', '', t['condition'] or ru_q, '',
                      '## Рубрика', '', '\n'.join(rub_lines) if rub_lines else '- Ответы: (ответ отсутствует в источнике)', '']
        else:
            ans = '\n'.join(r['text'] for r in t['rubric']).strip()
            core = t['condition'] if t['condition'] else ans
            if not core:
                core = '(ответ отсутствует в источнике)'
            parts += ['## Core Idea (RU)', '', core, '']
        fname = f"{oz_id} — {re.sub(r'[\\\\/:*?\"<>|]+',' ',ru_q)[:70].strip()}.md"
        open(os.path.join(OUT, fname), 'w', encoding='utf-8').write('\n'.join(parts))
        written += 1
    print('written:', written)

if __name__ == '__main__':
    main()
