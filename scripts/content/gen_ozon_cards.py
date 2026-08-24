#!/usr/bin/env python3
"""Generate bilingual vault cards from parsed Ozon Go tasks (63)."""
import json, os, re, sys

TASKS = json.load(open('/tmp/qb/ozon_tasks.json', encoding='utf-8'))
OUT = os.path.expanduser('~/developer/fluent-question-vault/Question Cards')

TOPIC_BY_SECTION = {
    'ОС, сети и эксплуатация': 'OS, Networking & Concurrency Fundamentals',
    'Базы данных': 'Databases & Data Modeling',
    'Архитектура': 'Architecture & Design Patterns',
    'Структуры данных': 'Algorithms',
}
GO_TOPIC = 'Go / Runtime & Language'   # new topic: >30 cards land here
LEVEL_BY_GRADE = {'17': 'Junior', '18': 'Middle', '19': 'Middle+', '20': 'Senior', '21': 'Senior'}

# Author-translated English questions, indexed by task order in the PDF.
EN = {
0:"What is a memory page?",
1:"What is a syscall?",
2:"Which OS synchronization primitives do you know?",
3:"What are user space and kernel space, and why does the separation exist?",
4:"What is a file descriptor?",
5:"A service on a bare server leaks memory. What happens when memory runs out?",
6:"How does a process differ from an OS thread?",
7:"What is GOMAXPROCS and why does it matter?",
8:"Conduct a code review of a developer's new cache intended for high-load production with a 20%/80% write/read split.",
9:"Explain how a hash table works: the general idea, collisions, and resizing.",
10:"Tell me about the hardest or most interesting engineering problem you have solved.",
11:"What does this Go program print (a buffered channel driven by select inside a loop)?",
12:"Explain how an LRU cache is built.",
13:"Design the schema for users, chats and messages: tables, keys and indexes.",
14:"Given an orders table, write the SQL queries required by the task.",
15:"What will this program print, and why?",
16:"What will this code print, and why?",
17:"What is the HTTP protocol made of: the main parts of an HTTP request and response?",
18:"Which tools do you use to monitor and debug microservices?",
19:"top shows a process consuming 146% CPU. Is that possible, and what should you do about it?",
20:"How do you kill a process on Linux?",
21:"How do TCP and UDP differ?",
22:"On the order-creation example: between request and response we also push items into analytics. Design this flow end to end.",
23:"Our service calls an external routing API whose plan limits RPS. Design a rate limiter.",
24:"Services A and B talk over HTTP. What options exist for scaling each side?",
25:"What is the difference between REST and RPC approaches?",
26:"What is the difference between a proxy and a reverse proxy?",
27:"A production table holds a billion rows and must stay live. How do you run a data fix modifying 10 million of them?",
28:"Which lock types exist in PostgreSQL, and who takes them?",
29:"Which anomalies can appear when several transactions run in parallel? Cover them briefly.",
30:"What is a transaction, and which isolation levels exist in RDBMS?",
31:"Build the optimal index for SELECT * FROM employee WHERE sex='m' AND salary>300000 AND age=20 ORDER BY created_at.",
32:"Which common constraint types exist in relational databases, and what is each for?",
33:"Model a library with Author, Book and Reader: exactly one physical copy that moves between readers.",
34:"Given a set of URLs, implement the concurrent checker described in the task.",
35:"We have a password database hashed with hashPassword and know the alphabet of possible characters. Implement the brute-force search the task describes.",
36:"What does this program print (loop variable capture with goroutines)?",
37:"Implement a simple in-memory cache library: unlimited memory, no TTL required.",
38:"This near-real cache wrapper from one of our services reads and writes through the cache yet makes things slower. Find out why.",
39:"Implement merge of N channels: the whole input stream is forwarded into one channel.",
40:"Will this program run correctly, and what does it print (unbuffered channel producer/consumer)?",
41:"Write a function returning an error without importing any package.",
42:"What is pprof and why is it needed?",
43:"What does this code print (ranging over a map)?",
44:"For `a := map[B]int{}` and `e, ok := a[d]`: when does this code error, and how do zero values and the comma-ok idiom behave here?",
45:"Will the AB→BC type assertion compile and succeed for *Foo implementing both? How are interfaces represented internally?",
46:"How do goroutines differ from threads?",
47:"What is defer and what does the sample print (deferred method receiver)?",
48:"What does this code print (goroutines capturing the range variable), and how do you fix it?",
49:"What is a closure? Give an example of its use.",
50:"What is a mutex and why is it needed?",
51:"What are channels and why do we need them?",
52:"Can you pass a variable into several goroutines? Show the buggy example — what happens?",
53:"What is a slice and how is it arranged internally?",
54:"What is a string in Go under the hood?",
55:"Given the database schema (users and purchases), write the SQL the task asks for.",
56:"Check whether a slice is monotonic (non-decreasing or non-increasing).",
57:"Given an int slice, implement remove deleting all zeros.",
58:"Implement zip joining elements of two slices into a slice of pairs.",
59:"Implement uniqRandn(n): a slice of n unique random numbers.",
60:"What will the program print and why (two channel receives evaluated together)?",
61:"What will the program print (string byte access and assignment attempt)?",
62:"What will the program print (append sharing the same backing array)?",
}

def ru_title(t):
    for block in t['condition']:
        if block and block != '-':
            return block.split('\n')[0][:90]
    return 'Задача Ozon Go'

def level_of(t):
    lv = [LEVEL_BY_GRADE.get(g) for g in t['grades'] if g in LEVEL_BY_GRADE]
    return lv[-1] if lv else ''

def group_of(t, i):
    sec = t['section']
    if sec == 'Архитектура':
        return 'System Design'
    if i == 10:
        return 'Behavioral'
    if i == 13:
        return 'System Design'
    if sec == 'Go (теория)':
        return 'Common Questions'
    if sec in ('Скрининг',):
        return 'Practical Tasks' if i in (14,55,56,57,58,59) else 'Common Questions'
    return 'Practical Tasks'

def topic_of(t, i):
    sec = t['section']
    if sec in TOPIC_BY_SECTION:
        return TOPIC_BY_SECTION[sec]
    if i in (10,):
        return 'Behavioral'
    if i in (14, 55):
        return 'Databases & Data Modeling'
    return GO_TOPIC

def sanitize(name):
    name = re.sub(r'[\\/:*?"<>|]+', ' ', name)
    return re.sub(r'\s+', ' ', name).strip(' .')[:80]

def main():
    assert len(TASKS) == 63 and len(EN) == 63
    written = []
    for i, t in enumerate(TASKS):
        oz_id = f'OZ-{101 + i}'
        ru_q = ru_title(t)
        cond = '\n\n'.join(b for b in t['condition'] if b and b != '-')
        rubric_lines = []
        for r in t['rubric']:
            label, text = r['label'], (r['text'] or '').strip()
            if label == 'note':
                continue
            if not text:
                text = '(ответ отсутствует в источнике)'
            flat_first, _, rest = text.partition('\n')
            rubric_lines.append(f'- {label}: {flat_first}')
            if rest:
                rubric_lines.append(rest)
        if not rubric_lines:
            rubric_lines.append('- Ответы: (ответ отсутствует в источнике)')
        timing = t.get('timing') or ''
        usage = t.get('usage') or ''
        meta = [
            f'ID: {oz_id}', 'Track: Backend', f'Topic: {topic_of(t, i)}',
            'Scope: Ozon', 'Lang:', 'Priority: common', f'Group: {group_of(t, i)}',
            f'Level: {level_of(t)}', 'Company: Ozon',
        ]
        if timing: meta.append(f'Timing: {timing}')
        if usage: meta.append(f'Usage: {usage}')
        meta.append(f'Question: {EN[i]}')
        parts = [f'# {oz_id} — {sanitize(ru_q)}', '\n'.join(meta),
                 '## Question (RU)', '', ru_q, '',
                 '## Task', '', cond.strip() or '(условие в источнике совпадает с вопросом)', '',
                 '## Рубрика', '', '\n'.join(rubric_lines), '']
        fname = f'{oz_id} — {sanitize(ru_q)}.md'
        open(os.path.join(OUT, fname), 'w', encoding='utf-8').write('\n'.join(parts))
        written.append(fname)
    print('written:', len(written))

if __name__ == '__main__':
    main()
