#!/usr/bin/env python3
import sys
sys.path.insert(0, 'scripts/content')
import gen_avito_cards as g

def en_for(title, EN):
    t = title.rstrip('?').strip()
    if t in EN: return EN[t]
    for k, v in EN.items():
        if t.startswith(k[:38]) or k.startswith(t[:38]):
            return v
    return None

def run(txt, ids, track, default_topic, topic_map, EN, group_fn=None):
    blocks = [b for b in g.parse_blocks(open(txt, encoding='utf-8').read()) if len(b['title']) >= 6]
    n = 0
    for b in blocks:
        i = ids[0]; ids[0] += 1
        en = en_for(b['title'], EN)
        topic = topic_map.get(b['title'], default_topic)
        group = group_fn(b) if group_fn else ('Common Questions' if b['kind'] == 'theory' else 'Practical Tasks')
        rub = g.rubric_from(b['lines']) if b['kind'] == 'theory' else None
        task = '\n'.join(b['lines']).strip() if b['kind'] == 'task' else None
        g.emit(f'AV-{i}', track, topic, group,
               '' if b['kind'] == 'theory' else g.LEVEL.get(b['diff'], ''),
               None if b['kind'] == 'theory' else g.DIFF.get(b['diff']),
               'Avito', en or b['title'], b['title'], task, rub,
               with_task=(b['kind'] == 'task'))
        n += 1
    return n

if __name__ == '__main__':
    ids = [101]
    # --- Go ---
    GO_EN = {
     'Указатели':"What does this Go program print — reassigning a pointer inside a function?",
     'Горутины в цикле':"What does this loop spawning goroutines print, and how do you make it deterministic?",
     'Сборка сниппета':"Assemble the given Go snippet so it compiles and produces the expected output.",
     'Прогноз погоды':"Implement the weather-forecast service described in the task.",
     'Сетевые запросы':"How do these Go network requests work, and what are the pitfalls?",
     'Параллельный запрос URL адресов':"Fetch a list of URLs concurrently in Go with bounded parallelism.",
     'Неблокирующий вызов функции':"Run a potentially blocking function without blocking the caller — which Go patterns apply?",
     'Что такое горутины в Go и как они соотносятся с потоками':"How do goroutines relate to operating-system threads?",
     'Чем занимается runtime в Go':"What does the Go runtime do?",
     'За счет чего Go позволяет работать с большим количеством сетевых соединений одновременно':"How does Go serve a huge number of concurrent network connections?",
     'Что такое каналы':"What are channels in Go?",
     'Расскажите о примитивах синхронизации и зачем они нужны':"Which synchronization primitives does Go provide, and why?",
     'Что такое интерфейс':"What is an interface in Go, and how is it represented internally?",
     'Использование памяти':"How do len and cap work for slices and maps, and how do allocation and GC behave?",
     'Написание тестов':"How do you test Go code: test kinds, mocks, and go test flags?",
     'Профилирование':"How do you profile Go programs with pprof and trace?",
    }
    n = run('/tmp/qb/avito/Go.txt', ids, 'Backend', 'Go / Runtime & Language', {}, GO_EN)
    print('Go:', n)
    # --- Java ---
    JAVA_EN = {
     'Autoboxing and unboxing':"Explain autoboxing/unboxing and the wrapper-class cache behavior proven by this test code.",
     'Java Streams':"Solve the Java Streams exercises and explain the pipeline.",
     'Функциональные интерфейсы':"Explain Java functional interfaces and implement the required ones.",
     'Double brace инициализация':"What is double-brace initialization, what does it compile to, and why is it an anti-pattern?",
     'Serialization':"How does Java serialization work for the case in the task?",
     'Garbage collector':"Walk through Java garbage collection as the task asks.",
     'Java Concurrency':"Solve the Java concurrency tasks and explain the primitives involved.",
     'Other tasks':"Solve the additional Java tasks.",
     'SQL практика':"Write the SQL queries required by the task.",
    }
    n = run('/tmp/qb/avito/Java.txt', ids, 'Backend', 'Java / Core Language',
            {'данных':'Java / Core Language'}, JAVA_EN)
    print('Java:', n)
    # --- Python ---
    PY_EN = {
     'Исключить один список из другого':"Implement excluding one list from another in Python.",
     'Классы в python':"Explain and complete the Python class task.",
     'Итераторы и генераторы':"Implement the iterator/generator task.",
     'Опрос сервиса':"Implement the service-polling client described in the task.",
     'Параметризованный декоратор':"Write a parameterized decorator per the task.",
     'Распараллелить cpu bound функцию':"Parallelize a CPU-bound Python function — why processes and not threads?",
     'транспортный уровень':"Answer the transport-level (TCP/UDP) questions from the task.",
    }
    PY_TOPIC = {
     'Исключить один список из другого':'Algorithms',
     'Классы в python':'Python / OOP & Data Model',
     'Итераторы и генераторы':'Python / Iterators & Generators',
     'Параметризованный декоратор':'Python / Metaprogramming',
     'Распараллелить cpu bound функцию':'Python / Concurrency & GIL',
     'транспортный уровень':'OS, Networking & Concurrency Fundamentals',
     'Опрос сервиса':'API Design',
    }
    n = run('/tmp/qb/avito/Python.txt', ids, 'Backend', 'Python / OOP & Data Model', PY_TOPIC, PY_EN)
    print('Python:', n)
    # --- Frontend ---
    FE_EN = {
     'Замыкание / Область видимости':"Solve the closure/scope task and explain the output.",
     '(OLD) Event Loop':"Predict the asynchronous ordering in this legacy event-loop task.",
     'Event Loop':"Predict the console.log ordering — event loop phases and microtasks.",
     'Области видимости':"Solve the scoping puzzle and explain why.",
     'Jquery':"Solve the jQuery task.",
     'Работа браузера':"Answer the browser-internals questions from the task.",
     'Сеть':"Answer the networking questions from the task.",
     'Сеть - Клиент для получения фича-тогглов':"Build a feature-toggle client with periodic refresh and error handling.",
     'Сеть - Клиент для отправки аналитики':"Build an analytics client with a batching queue.",
     'Оптимизация':"Solve the frontend optimization task.",
     'Работа с DOM – Изображения':"Solve the DOM images task (loading/optimization).",
    }
    FE_TOPIC = {
     'Замыкание / Область видимости':'Node / JS Fundamentals',
     '(OLD) Event Loop':'Node / Event Loop & Scheduling',
     'Event Loop':'Node / Event Loop & Scheduling',
     'Области видимости':'Node / JS Fundamentals',
     'Jquery':'JavaScript / DOM Events',
     'Работа браузера':'Networking / Browser',
     'Сеть':'Networking / Browser',
     'Сеть - Клиент для получения фича-тогглов':'API Design',
     'Сеть - Клиент для отправки аналитики':'API Design',
     'Оптимизация':'Frontend Performance',
     'Работа с DOM – Изображения':'JavaScript / DOM Events',
    }
    n = run('/tmp/qb/avito/Frontend.txt', ids, 'Frontend', 'Node / JS Fundamentals', FE_TOPIC, FE_EN,
            group_fn=lambda b: 'Practical Tasks')
    print('Frontend:', n)
