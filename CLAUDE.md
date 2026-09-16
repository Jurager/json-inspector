# JSON Inspector

Десктопный клиент для JSON:API: Wails v3 (Go) + Vue 3. Вся логика — в Go, Vue только рисует.

## Перед первой правкой

**Прочитать `docs/ARCHITECTURE.md`.** Там слои, правила нейминга и пошаговый рецепт: как добавить
фичу так, чтобы она легла в структуру — `domain` → порт → `usecase` → `infra`/`transport` → зеркало
во фронте. Слои проверяются тестом `internal/archtest`: нарушение валит `go test`.

## Команды

```bash
task dev                                  # запуск с hot reload
task build                                # сборка под текущую ОС
go vet ./... && go test ./...              # включая archtest и тесты миграций
wails3 generate bindings -clean=true -ts -i   # ОБЯЗАТЕЛЬНО после правки Go-методов
cd frontend && npx vue-tsc --noEmit        # типы фронта
```

Переименование или удаление привязанного Go-метода ломает фронт **в рантайме** (вызов идёт по
числовому id), `vue-tsc` этого не поймает. Поэтому биндинги регенерируются в том же изменении.

## Правила проекта

- **Комментарии объясняют «почему», а не «что»**; строка ≤ 100 символов. Историю правок не пишем —
  её пишет git.
- **Вид сверяется по макету** `design_handoff_json_inspector/JSON Inspector - Зоны и редизайн.dc.html`
  (числа там точные; при расхождении с README верить макету). Новых цветовых токенов не заводить.
- **Поведение проверяется настоящими событиями.** Синтетические `dispatchEvent` reka-ui **не
  принимает**: нужен Chrome с `--remote-debugging-port` и `Input.dispatchMouseEvent` через CDP.
  Для расширения — headless не годится, `Extensions.loadUnpacked` по CDP.
- **Секреты лежат в SQLite открытым текстом** — сознательное решение ради переносимости (связки
  ключей нет). Поэтому: `0600` на базу и её `-wal`/`-shm`, пометка в UI, значения не уезжают в
  экспорт, в логи и в DTO (только `hasValue` + явный `Reveal`).
- **Тесты рядом с кодом**, на сценарий — с фейковым портом, без окна и базы.

## Где что лежит

| Что | Где |
|---|---|
| База | `platform.DataDir()` → `os.UserConfigDir()/json-inspector/app.db` |
| Схема | `migrations/NNN_*.sql`, раннер — `pkg/migrate` |
| Привязанные к фронту сервисы | `internal/transport/wails/services_*.go` |
| Имена и типы событий | `internal/transport/wails/events.go` (единственное место) |
| Движок запросов | `internal/infra/httpx` |
| Песочница скриптов | `internal/infra/scriptengine` (goja) |
| Макет | `design_handoff_json_inspector/` |
