#!/usr/bin/env python3
"""Repair Russian prompts that came out of source layout instead of authoring.

Seventy imported cards carry a Russian "question" that is really a PDF section
heading, a Notion page title, or a stray glyph — "C", ":", "SQL", "Указатели",
"DeepCopy". The English side of each is a proper question, so the fix is to
author the Russian question and write it into the vault, which is where a card's
text is supposed to live.

Two cards (av-124, av-146) lost their English prompt to the same extraction, so
those get both sides.

Writing into the vault matters more than patching the database: a projection-only
repair survives until the next clean rebuild and then silently reverts.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

VAULT = Path.home() / "developer" / "fluent-question-vault"

# stable-key suffix -> Russian question. English prompt is authoritative; these
# are written from it, not machine-translated from the broken Russian.
RU: dict[str, str] = {
    # --- Avito, Go ---
    "av-101": "Что выведет эта Go-программа, если переприсвоить указатель внутри функции?",
    "av-102": "Что выведет цикл, порождающий горутины, и как сделать вывод детерминированным?",
    "av-103": "Соберите приведённый сниппет на Go так, чтобы он компилировался и давал ожидаемый вывод.",
    "av-104": "Реализуйте сервис прогноза погоды: расчёт занимает около секунды, а ручка должна держать 10k RPS.",
    "av-105": "Как работают эти сетевые запросы на Go и какие здесь подводные камни?",
    "av-111": "Что такое каналы в Go?",
    "av-113": "Что такое интерфейс в Go и как он устроен внутри?",
    "av-114": "Чем различаются len и cap у слайсов и map, как происходит выделение памяти и сборка мусора?",
    "av-115": "Как тестировать код на Go: какие виды тестов, что такое моки, какие флаги у go test?",
    "av-116": "Как профилировать программы на Go с помощью pprof и trace?",
    # --- Avito, Java ---
    "av-117": "Объясните autoboxing и unboxing и поведение кэша классов-обёрток, которое доказывает этот тест.",
    "av-118": "Решите задачи на Java Streams и объясните, как устроен конвейер.",
    "av-121": "Как работает сериализация в Java для случая из задачи?",
    "av-122": "Разберите работу сборщика мусора в Java так, как требует задача.",
    "av-123": "Решите задачи на конкурентность в Java и объясните использованные примитивы синхронизации.",
    "av-124": "Реализуйте класс поиска минимального значения в окне потоковых данных: какова сложность и сколько нужно дополнительной памяти?",
    "av-125": "Решите дополнительные задачи на Java.",
    "av-126": "Напишите SQL-запросы, которые требует задача.",
    # --- Avito, Python ---
    "av-128": "Объясните и допишите задачу на классы в Python.",
    "av-129": "Реализуйте задачу на итераторы и генераторы.",
    "av-130": "Реализуйте клиент опроса сервисов, описанный в задаче.",
    "av-134": "Ответьте на вопросы по транспортному уровню — TCP и UDP — из задачи.",
    # --- Avito, Frontend ---
    "av-136": "Предскажите порядок асинхронного вывода в этой задаче на event loop.",
    "av-137": "Предскажите порядок вызовов console.log — фазы event loop и микрозадачи.",
    "av-138": "Что выведет эта головоломка на замыкания с modifyItemData и printItemData?",
    "av-139": "Разберите функцию makeScopedLogger: чему равны count и value в каждой точке логирования?",
    "av-140": "Напишите функцию $, чтобы методы можно было вызывать цепочкой.",
    "av-141": "Ответьте на вопросы о внутреннем устройстве браузера из задачи.",
    "av-144": "Решите задачу на оптимизацию фронтенда: как ускорить загрузку изображений и кода?",
    "av-146": "Реализуйте виджет саджестов поиска: запрос по введённому тексту, отрисовка списка и обработка ошибок.",
    # --- Notion, JS ---
    "nt-601": "Реализуйте функцию глубокого копирования объекта.",
    "nt-602": "Реализуйте класс EventEmitter.",
    "nt-603": "Напишите адаптер promisify для функций с колбэком в стиле error-first.",
    "nt-604": "Что выведет setTimeout внутри цикла и почему?",
    "nt-606": "Реализуйте паттерн «Абстрактная фабрика».",
    "nt-607": "Реализуйте функцию memoize.",
    "nt-609": "Реализуйте декоратор, описанный в задаче.",
    "nt-610": "Реализуйте drag and drop так, как описано в задаче.",
    "nt-611": "Сделайте элемент перетаскиваемым согласно задаче.",
    "nt-612": "Реализуйте обёртку над fetch с повторными попытками.",
    "nt-613": "Объясните class properties так, как требует задача.",
    "nt-614": "Реализуйте userService, описанный в задаче.",
    "nt-615": "Реализуйте калькулятор, описанный в задаче.",
    "nt-617": "В каком порядке выведутся микрозадачи и макрозадачи?",
    "nt-618": "Реализуйте Promise.all с нуля.",
    "nt-619": "Реализуйте Promise.any с нуля.",
    "nt-620": "Ответьте на вопросы фронтенд-скрининга 2025 года.",
    "nt-621": "Реализуйте фабрику счётчиков, описанную в задаче.",
    # --- Ozon, ОС и Go ---
    "oz-101": "Что такое страница памяти?",
    "oz-102": "Что такое системный вызов?",
    "oz-112": "Что выведет эта Go-программа — буферизированный канал, читаемый через select в цикле?",
    "oz-115": "По таблице orders напишите SQL-запросы, которые требует задача.",
    "oz-120": "top показывает, что процесс потребляет 146% CPU. Такое возможно и что с этим делать?",
    "oz-135": "Дан набор URL — реализуйте конкурентную проверку, описанную в задаче.",
    "oz-144": "Что выведет этот код при обходе map через range?",
    "oz-145": "Для `a := map[B]int{}` и `e, ok := a[d]` — когда этот код даст ошибку и как ведут себя нулевые значения и сравнение?",
    "oz-146": "Скомпилируется ли и сработает ли приведение типа AB→BC для *Foo, реализующего оба интерфейса? Как интерфейсы устроены внутри?",
    "oz-149": "Что выведет этот код с горутинами, захватывающими переменную цикла, и как это исправить?",
    "oz-162": "Что выведет программа при обращении к байту строки и попытке присваивания?",
    "oz-163": "Что выведет программа, где два append разделяют один и тот же базовый массив?",
    # --- Ozon, архитектура ---
    "oz-168": "Что такое Domain-Driven Design?",
    "oz-169": "Как устроены колоночные базы данных и когда они уместны?",
    "oz-170": "Сделайте обзор нереляционных баз данных: какие бывают и зачем нужны?",
    "oz-173": "Какие подходы к инвалидации кэша существуют?",
    "oz-174": "Какие стратегии кэширования вы знаете?",
    "oz-177": "Что такое идемпотентность и как её реализовать?",
    "oz-178": "Какие алгоритмы балансировки нагрузки вы знаете?",
    "oz-179": "Что такое sticky sessions и зачем они нужны?",
    "oz-188": "Что такое Сага?",
    # --- System design screening ---
    "sd-208": "Что такое SLA?",
}

# Cards whose English prompt was also lost to extraction.
EN: dict[str, str] = {
    "av-124": "Implement a sliding-window minimum over streaming data: what is the complexity and how much extra memory does it need?",
    "av-146": "Implement a search suggest widget: query on input, render the list, and handle errors.",
}

SECTION = "## Question (RU)"


def find_card(key: str) -> Path | None:
    prefix = key.upper() + " "
    for path in (VAULT / "Question Cards").glob("*.md"):
        if path.name.startswith(prefix):
            return path
    return None


def patch(path: Path, ru: str, en: str | None) -> str:
    text = path.read_text(encoding="utf-8")

    if en:
        text, n = re.subn(r"^Question:.*$", f"Question: {en}", text, count=1, flags=re.M)
        if not n:
            return "no Question: header"

    block = f"{SECTION}\n\n{ru}\n"
    if SECTION in text:
        # Replace the existing section body up to the next heading.
        text = re.sub(
            rf"^{re.escape(SECTION)}\n(?:(?!^## ).*\n)*",
            block,
            text,
            count=1,
            flags=re.M,
        )
        outcome = "replaced"
    else:
        # Insert before the first section so it reads right after the metadata.
        first = re.search(r"^## ", text, flags=re.M)
        if not first:
            return "no sections"
        text = text[: first.start()] + block + "\n" + text[first.start() :]
        outcome = "inserted"

    path.write_text(text, encoding="utf-8")
    return outcome


def main() -> int:
    missing, results = [], {}
    for key, ru in RU.items():
        path = find_card(key)
        if path is None:
            missing.append(key)
            continue
        results[key] = patch(path, ru, EN.get(key))

    for key, outcome in sorted(results.items()):
        if outcome in {"replaced", "inserted"}:
            continue
        print(f"  ! {key}: {outcome}")

    print(f"patched: {sum(1 for v in results.values() if v in {'replaced', 'inserted'})}/{len(RU)}")
    if missing:
        print(f"card file not found: {', '.join(missing)}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
