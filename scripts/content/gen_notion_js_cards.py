#!/usr/bin/env python3
import os, re, html, glob

from quality import require_clean_text, require_prompt

NB = "/Users/sergeyzhechko/Downloads/Банк вопросов/Желтый Банк/Секции/Секции по языкам платформам/JavaScript/Вопросы — JS"
V = os.path.expanduser('~/developer/fluent-question-vault/Question Cards')

def extract(path):
    raw = open(path, encoding='utf-8').read()
    m = re.search(r'<div class="page-body">(.*?)<footer', raw, re.S) or re.search(r'<div class="page-body">(.*)', raw, re.S)
    body = m.group(1) if m else raw
    body = re.sub(r'<pre[^>]*>(.*?)</pre>',
                  lambda mm: '\n```\n' + html.unescape(re.sub(r'<[^>]+>', '', mm.group(1))) + '\n```\n',
                  body, flags=re.S)
    txt = re.sub(r'<[^>]+>', '\n', body)
    txt = html.unescape(txt)
    lines = [l.strip() for l in txt.split('\n') if l.strip()]
    sections, cur = {}, '_preamble'
    for ln in lines:
        if ln in ('Сложность', 'Тэги', 'Задача', 'Описание задачи', 'Решение', 'Фидбек', 'Источник', 'Собеседования'):
            cur = ln; sections.setdefault(cur, []); continue
        sections.setdefault(cur, []).append(ln)
    return sections

EN = {
 'Порядок микротасок':"In what order do the microtask queue and macrotasks print?",
 'Порядок вызова console log':"In what order do these console.log calls execute?",
 'Скрининг 2025':"Answer the 2025 frontend screening questions.",
 'Калькулятор':"Implement the calculator described in the task.",
 'Создание counter':"Implement the counter factory described in the task.",
 'Реализовать Promise all':"Implement Promise.all from scratch.",
 'Реализовать Promise any':"Implement Promise.any from scratch.",
 'Написать пример наследования':"Write an example of inheritance in JavaScript.",
 'promisify':"Write a promisify adapter for error-first callback functions.",
 'setTimeout в цикле':"What does setTimeout inside a loop print, and why?",
 'Освобождаемые события (Disposable)':"Implement disposable event subscriptions (cleanup pattern).",
 'AbstractFactory':"Implement the Abstract Factory pattern per the task.",
 'memoize':"Implement the memoize helper described in the task.",
 'class properties':"Explain class properties as required by the task.",
 'Decorator':"Implement the decorator described in the task.",
 'Drag and drop':"Implement drag and drop as described in the task.",
 'Draggble':"Make the element draggable per the task.",
 'Fetch Retry':"Implement a fetch wrapper with retries per the task.",
 'DeepCopy':"Implement a deep copy function per the task.",
 'EventEmitter':"Implement the EventEmitter class per the task.",
 'userService':"Implement the userService described in the task.",
}
TOPIC = {
 'Порядок микротасок':'Node / Event Loop & Scheduling',
 'Порядок вызова console log':'Node / Event Loop & Scheduling',
 'setTimeout в цикле':'Node / Event Loop & Scheduling',
 'Скрининг 2025':'Node / JS Fundamentals',
 'Калькулятор':'Node / JS Fundamentals',
 'Создание counter':'Node / JS Fundamentals',
 'Написать пример наследования':'Node / JS Fundamentals',
 'Освобождаемые события (Disposable)':'Node / JS Fundamentals',
 'memoize':'Node / JS Fundamentals',
 'class properties':'Node / JS Fundamentals',
 'DeepCopy':'Node / JS Fundamentals',
 'Реализовать Promise all':'Node / Async & Promises',
 'Реализовать Promise any':'Node / Async & Promises',
 'promisify':'Node / Async & Promises',
 'Fetch Retry':'Node / Async & Promises',
 'EventEmitter':'Node / Async & Promises',
 'userService':'Node / Async & Promises',
 'AbstractFactory':'Architecture & Design Patterns',
 'Decorator':'Architecture & Design Patterns',
 'Drag and drop':'JavaScript / DOM Events',
 'Draggble':'JavaScript / DOM Events',
}
LV = {'EASY':'Junior','MEDIUM':'Middle','HARD':'Senior'}

files = sorted(glob.glob(NB + '/*.html'))
n = 601
for path in files:
    base = os.path.basename(path)
    if re.match(r'^Вопросы — JS [0-9a-f]+\.html$', base):
        continue
    title = re.sub(r'\s*[0-9a-f]{32}\.html$', '', base).strip()
    sec = extract(path)
    diff = (sec.get('Сложность') or [''])[0]
    level = LV.get(diff.upper(), '')
    tags = ', '.join(sec.get('Тэги', []))
    task = '\n'.join(sec.get('Задача', []) + sec.get('Описание задачи', []))
    solution = '\n'.join(sec.get('Решение', []))
    key = re.sub(r'^\[[^\]]+\]\s*', '', title)
    en = EN.get(key) or EN.get(title) or key
    source = f'Notion:NT-{n}'
    topic = TOPIC.get(key, 'Node / JS Fundamentals')
    require_prompt(en, source=source, field='Question (EN)', title=key, topic=topic)
    require_prompt(key, source=source, field='Question (RU)', title=key, topic=topic)
    require_clean_text(task, source=source, field='Task')
    require_clean_text(solution, source=source, field='Solution')
    meta = [f'ID: NT-{n}', 'Track: Frontend', f'Topic: {TOPIC.get(key, "Node / JS Fundamentals")}',
            'Scope: Notion', 'Lang:', 'Priority: common', 'Group: Practical Tasks',
            f'Level: {level}', 'Company: Avito', f'Difficulty: {diff or ""}',
            f'Question: {en}']
    if tags: meta.append(f'Tags: {tags}')
    parts = [f'# NT-{n} — {key}', '\n'.join(meta),
             '## Question (RU)', '', key, '',
             '## Task', '', task or key, '']
    if solution:
        parts += ['## Solution', '', solution, '']
    fb = '\n'.join(sec.get('Фидбек', []))
    if fb:
        parts += ['## Walkthrough', '', fb, '']
    open(os.path.join(V, f'NT-{n} — {key[:70]}.md'), 'w', encoding='utf-8').write('\n'.join(parts))
    n += 1
print('written:', n - 601)
