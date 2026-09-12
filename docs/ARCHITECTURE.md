# Архитектура

Как устроено приложение, почему именно так и **как добавить новую фичу**. Читать перед первой
правкой в Go и перечитывать, если рецепт разошёлся с кодом — тогда правится документ.

## Зачем всё в Go

Раньше логика жила в TypeScript: два Pinia-стора держали историю, перехват, черновик, окружения и
секреты, четыре ключа `localStorage` были единственным хранилищем, а Go отправлял запросы и
обслуживал мост расширения. Отсюда росли проблемы: хранилище — это blob в профиле WebView (нет
выборок, нет ретенции, тела всех записей тянутся в UI целиком), секреты держались на связке ключей
macOS и на Windows/Linux не работали, форма перехваченного запроса была продублирована в трёх
местах, а тестов не было ни одного.

Теперь единственный источник правды — Go: сеть, хранилище (SQLite + миграции), окружения и
переменные с секретами, история и перехват, коллекции, разбор документов и команд, пользовательские
скрипты. Vue рисует и держит только состояние вида.

## Слои

Стрелка — «имеет право импортировать». Правило проверяется тестом `internal/archtest`, а не
договорённостью: нарушение валит сборку.

```
main.go ──► transport/ ──► usecase/ ──► domain/
                │             │
                └──► infra/ ◄─┘
                       ▲
  vars/ command/ dotenv/ postman/ jsonapi/ — чистые, только stdlib
```

| Слой | Можно импортировать | Нельзя |
|---|---|---|
| `internal/domain` | stdlib | ничего своего — вообще |
| `internal/usecase/<фича>` | `domain`, свои интерфейсы, чистые пакеты | другие фичи, `infra`, `transport`, Wails |
| `internal/infra/*` | `domain`, чистые пакеты, драйверы | `usecase`, `transport`, Wails |
| `internal/transport/*` | всё | — (здесь композиция) |
| `internal/platform` | stdlib | всё остальное |

Два правила, которые снимают большинство вопросов «куда это положить»:

- **Фичи не импортируют друг друга.** Нужна чужая фича — объявляешь у себя маленький интерфейс
  (`VarResolver`, `RecordWriter`) и получаешь реализацию через fx. Порт объявляет **потребитель**.
  Так цикл `drafts ↔ collections` ловится сразу, а не через полгода.
- **Внешний мир — это пакет в `infra/` или `transport/`.** Новый протокол, драйвер, сервис — новый
  пакет там, а не метод в сценарии.

## Дерево

```
main.go                       сборка: fx + Wails, без логики
migrations/0001_*.sql …       схема, по файлу на область + embed.go
internal/
  domain/                     сущности, правила, ошибки; ноль импортов проекта
  usecase/                    сценарии: папка на фичу, фасад — usecase.go
  infra/
    sqlite/                   Store: соединение, прагмы, миграции
    httpx/                    движок запросов
    keychain/                 связка ключей (уходит после импорта секретов)
    updater/                  самообновление
  transport/
    wails/                    привязанные сервисы, события, окна, меню
    bridge/                   WebSocket-сервер расширения
  migrate/                    раннер миграций + testdata
  vars/ command/ dotenv/ postman/ jsonapi/    чистые алгоритмы, тесты рядом
  platform/                   пути, id, идентичность сборки
  archtest/                   правило зависимостей и нейминга тестом
```

### Состояние на сегодня

Готово: `platform`, `domain` (ошибки, `Response`), `infra/sqlite` + миграции 0001–0006 + `migrate` с
тестами, `infra/httpx` (движок в дорефакторной форме), `infra/keychain`, `infra/updater`,
`transport/bridge` (с интерфейсом `Ingest`), `transport/wails` (SystemService, RequestsService,
EnvironmentsService, BridgeService, Host, шина событий, экран ошибки старта), `archtest`.

Дальше по плану: `usecase/*` появляется вместе с первой переехавшей фичей (окружения, история,
черновик), `transport/wails` растёт сервисами по агрегатам, `frontend/src/ipc/` — тонкий
типизированный клиент событий.

## Нейминг

- **Пакет называет то, что даёт.** Никаких `util`, `common`, `helpers`, `models`, `types`,
  `constants` — такие имена запрещены и проверяются тестом.
- **Файл — роль в фиче**: `usecase.go` — фасад пакета, `transform.go`/`preview.go` — части одной
  ответственности, `<имя>_test.go` — тесты рядом.
- **Один агрегат — одна фича-папка.** Захотелось вторую ответственность — отдельная фича или
  отдельный пакет, а не 34-й файл в `request`.
- **`module.go` — по одной строке на пакет** (`var Module = fx.Module(…)`); агрегат фич собирает
  `usecase/module.go`. Новая фича = папка + строка в агрегате, `main.go` не трогается.
- **Ошибки доменные**: `errors.Is(err, domain.ErrNotFound)`; `sql.ErrNoRows` наружу не течёт.
- **Имена наружу** (в TS) берутся из `domain`, без префиксов-эхо.

## Как добавить фичу

1. **Понятия и правила** — в `domain/`. Если появилось новое понятие, там же его тип и инварианты.
2. **Порт** — интерфейс нужного тебе I/O объявляешь **у себя**: `usecase/<фича>/ports.go`. Реализация
   ляжет в `infra/`, но знать об этом сценарий не должен.
3. **Сценарии** — папка `usecase/<фича>/` с `usecase.go`-фасадом. Чужая фича — через свой интерфейс.
4. **Хранение** — миграция `migrations/NNNN_*.sql` + файл `<агрегат>_store.go` в `infra/sqlite`.
5. **Наружу** — метод в `transport/wails/services_<фича>.go`; нужен push — тип и имя события в
   `events.go` (там же `application.RegisterEvent[...]`).
6. **Во фронте** — только зеркало: поле в сторе + применение события. Логики нет.

### Пример: «закрепить запись в истории»

```go
// 1. domain/record.go — понятие и правило живут рядом.
type RecordSummary struct {
    ID      string
    Pinned  bool
    // …
}

// 2. usecase/record/ports.go — что сценарию нужно от хранилища. Объявляет потребитель.
type RecordStore interface {
    SetPinned(ctx context.Context, id string, pinned bool) error
    ListRecent(ctx context.Context, limit int) ([]domain.RecordSummary, error)
}

// 3. usecase/record/usecase.go — фасад пакета.
func (u *UseCase) Pin(ctx context.Context, id string, pinned bool) error {
    if err := u.store.SetPinned(ctx, id, pinned); err != nil {
        return fmt.Errorf("pinning record %s: %w", id, err)
    }
    u.notify.Publish(events.RecordChanged, domain.RecordChanged{ID: id, Pinned: pinned})
    return nil
}
```

```sql
-- 4. migrations/0007_records_pinned.sql
ALTER TABLE records ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
```

```go
// 5. transport/wails/services_record.go — привязка, и ничего кроме неё.
func (s *RecordsService) PinRecord(ctx context.Context, id string, pinned bool) error {
    return s.records.Pin(ctx, id, pinned)
}
```

```ts
// 6. frontend/src/ipc/ — зеркало. Типы приходят из биндингов, руками не пишутся.
onRecordChanged((e) => {
  const record = records.value.find((r) => r.id === e.id)
  if (record) record.pinned = e.pinned
})
```

Проверки: `go test ./internal/archtest/...` (слои не поехали), `wails3 generate bindings -clean=true
-ts -i` (иначе фронт не увидит метод), тест на сценарий с фейковым хранилищем.

### Чек-лист ревью

- [ ] Фича не импортирует другую фичу; чужое — через свой интерфейс.
- [ ] Новый внешний мир — новый пакет в `infra/` или `transport/`, а не метод в сценарии.
- [ ] DTO объявлен один раз, в `domain`; рукописных копий в TS нет.
- [ ] У каждого события есть тип в `RegisterEvent`, имя — только в `events.go`.
- [ ] `domain` не знает про SQL, окна и HTTP.
- [ ] Тесты рядом с кодом; на сценарий — с фейковым портом, без окна и базы.
- [ ] `wails3 generate bindings` выполнен, диф биндингов в коммите.

## Что остаётся во Vue

Состояние вида: активная вкладка ответа, раскрытый поповер, текст поиска, выделение в дереве,
каретка и скролл. Анимации (`themeWipe.ts`, `useHoverArrival`, `useResizableWidth`),
`clipboard.ts`, локализованные строки (`format.ts`), подсветка JSON (`highlightJson` — это HTML),
скрипт темы до первой отрисовки в `index.html`/`about.html` и весь рендер.

Pinia остаётся тонким зеркалом: поля приходят из снимка Go и обновляются событиями, вычислений там
нет. Один враппер `frontend/src/ipc/` — единственное место, которое знает имена событий и содержит
`as`.

## Чего не делаем

- Репозиторий-интерфейс на каждый агрегат при единственной реализации: интерфейс появляется, когда
  есть фейк в тесте или второй потребитель.
- Вынос тел на диск (колонка `file_path` зарезервирована, в v1 — лимит 8 МиБ с `truncated`).
- Пул `http.Client` под профили, пока нет UI настроек прокси.
- Мок-слой сервисов для запуска UI без Go — вернёмся, если понадобится, и это будет
  `transport/mock`, а не второй набор сценариев.
- `pm.sendRequest`, `setTimeout`, клон chai, полноценный Postman v2.1, дифф ответов до появления UI.
- `fx.Annotate`, именованные значения, `fx.Module` на каждый чих: обычные конструкторы + один `fx.In`.
- Аккаунт, синхронизация, шаринг: места в макете есть, модели данных нет. Схема готова принять
  `workspace_id`/`updated_at` миграцией — на этом останавливаемся.

## Проверка

```bash
go vet ./... && go test ./...        # включая archtest, миграции, Store
task build                            # сборка приложения под текущую ОС
task dev                              # запуск с hot reload
wails3 generate bindings -clean=true -ts -i    # после любой правки Go-методов
cd frontend && npx vue-tsc --noEmit   # типы фронта против биндингов
```

Ловушки, на которые уже наступали:

- **Переименование привязанного метода ломает фронт в рантайме**, а не при сборке: биндинги
  вызываются по числовому id, и `vue-tsc` этого не видит. Поэтому `generate bindings` — обязательный
  шаг, а не «когда вспомню».
- **Прагмы SQLite живут в DSN.** `foreign_keys` действует на соединение, а пул выдаёт разные, так что
  `db.Exec("PRAGMA …")` молча отключает все `ON DELETE CASCADE`. Тест `store_test.go` проверяет
  именно то соединение, которое выдаёт пул.
- **До `Run()` у Wails нет реализации окна**, поэтому `app.Dialog` паникует. Ошибка старта базы
  запоминается в `Status` и рисуется фронтом на урезанном наборе сервисов.
