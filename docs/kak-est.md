# Как есть

Конспект тестового сервиса переводов монет. Сверено с PDF «Go Code Challenge 3. Сервис переводов между аккаунтами» (4 страницы).

Ограничения этой работы: один процесс, только REST на gin, одна база Postgres.

## Что в PDF

Игроки переводят внутриигровые монеты. В тексте это нужно «для обмена предметами или подарков друзьям». Контракт описывает только монеты. Количества предметов на продажу в PDF нет: ни в запросе, ни в ответе, ни в начальных данных.

`POST /api/v1/transfers`. Тело: `idempotency_key`, `from_account_id`, `to_account_id`, `amount`.

Успех, HTTP 200:

```json
{
  "transfer_id": "...",
  "status": "completed",
  "from_account_id": "acc_1",
  "to_account_id": "acc_2",
  "amount": 100
}
```

Отказ, HTTP 422: `status` строго `rejected`, `reason` строго одна из строк `insufficient funds`, `self-transfer not allowed`, `account not found`.

`GET /api/v1/accounts/{id}`. Успех 200: `account_id`, `balance`. Нет аккаунта — 404.

`GET /api/v1/transfers/{idempotency_key}`. Успех 200: `transfer_id`, `idempotency_key`, `from_account_id`, `to_account_id`, `amount`, `status`, `created_at`. Нет перевода — 404.

Правила:

- Баланс не становится отрицательным. Иначе 422 и `insufficient funds`.
- Тот же `idempotency_key` и то же тело: тот же ответ, перевод не применяется второй раз.
- Тот же `idempotency_key` и другие `from_account_id`, `to_account_id` или `amount`: 409.
- `from_account_id` равен `to_account_id`: 422 и `self-transfer not allowed`.
- Аккаунта нет: 422 и `account not found`.
- Пустые `idempotency_key`, `from_account_id`, `to_account_id`: 400.
- Старт: `acc_1` … `acc_10`, у каждого баланс 1000. Сумма системы 10000.
- `docker-compose.yml`, сервис на `http://localhost:3000/`.
- Тесты на значимых случаях, не на полном покрытии.
- Краткое описание: что менять для продакшена, почему база и библиотеки, инфраструктура и масштаб, компромиссы. Черновик письма — `docs/kak-nado.md`.
- Базу PDF оставляет на выбор. Нагрузку, реплики, очередь, уведомления и вебсокеты PDF не задаёт.

## Принятые отличия

У аккаунта в этом сервисе один счётчик — баланс монет. Количество предметов на продажу описано в `docs/kak-nado.md`.

- `created_at` в `TransferResponse` — nullable. Заполненное значение — время строки `TransferModel`, RFC3339, UTC. Рядом в ответе `idempotency_key`, `status_code` и `reason`, когда `GetTransfer` читает записанный отказ.
- `amount` и `balance` — JSON-числа. Ноль, отрицательное число и больше двух знаков после запятой — 400. `1` и `1.2` принимаются и хранятся как `1.00` и `1.20`.
- Оригинальный запрос — строка `TransferModel`. В ней либо исполненный перевод, либо нехватка средств. Отдельной модели идемпотентности нет. Повтор того же `idempotency_key` и того же тела возвращает эту строку и перевод второй раз не проводит.
- Сериализация и взаимная блокировка откатывают транзакцию перевода. Строка не пишется. Сервис транзакцию заново не открывает. Ответ один: 409 `transfer transaction rolled back, retry the request`. Повтор решает клиент. Уже занятый `transfer_id` — 409 `transfer_id already exists`. Нет соединения с Postgres — 500 `postgres connection failed`.
- `self-transfer not allowed` и `account not found` в `TransferModel` нет. Повтор того же ключа после них — новый запрос. `reason` один — `account not found`: `PostTransfer` отвечает 422 `rejected`, `GetAccount` отвечает 404 `not found`.
- Тела 400 и 409 задаёт реестр ошибок. Другое тело уже записанного ключа — 409 `idempotency key reused with different payload`.
- `idempotency_key` — UUID. Он же `transfer_id`. Сервис UUID не генерирует.
- Начальные `acc_1` … `acc_10` создаёт флаг конфигурации: локально включён, в продовом запуске выключен. Повторный старт балансы не меняет.

## Структура проекта

Три слоя. Верхний не кладёт в бизнес указатели JSON и `sql.Null`. Репозиторий видит только модели.

| Слой | Каталог | Что здесь |
|---|---|---|
| Транспорт | `internal/handler`, `internal/handler/request`, `internal/handler/response` | gin, разбор запроса, сбор ответа, комментарии swag |
| Бизнес | `internal/services`, `internal/services/structures` | правила перевода. Типы без суффиксов: `Transfer`, `Account` |
| Репозиторий | `internal/repositories`, `models` | `AddModel`, `FindModel`, `UpdateModel`. SQL через squirrel |

Рядом, не как четвёртый слой правил:

| Каталог | Что здесь |
|---|---|
| `errors` | `errors.Response` и реестр. Наружу ошибку пишет хендлер |
| `internal/config` | флаг начальных аккаунтов |
| `deployments` | локальный запуск и продовый |
| `migrations` | Flyway, один файл на одну задачу, имена вида `V1_001__init.sql` |
| `docs` | этот конспект, `docs/kak-nado.md`, `docs/swagger.yaml` |

Обычный `Find` не открывает транзакцию и не берёт `FOR UPDATE`. Списание идёт своей транзакцией `REPEATABLE READ` с `SELECT … FOR UPDATE` исходящего аккаунта. Тесты репозитория: Postgres из compose, миграция, свои данные, затем Flyway clean. Clean разрешён только в тестовом прогоне. Утверждения — testify.

## Бизнес-логика

`TransferID` приходит в запросе. Это тот же UUID, что `idempotency_key`.

Сумма в сервисе — `decimal`. В Postgres колонка `numeric(20,2)`. На проводе `amount` и `balance` — JSON-числа. Не больше двух знаков после запятой, строго больше нуля. `1` и `1.2` до сервиса становятся `1.00` и `1.20`. Иначе хендлер отвечает 400 и сервис не вызывается. Число знаков смотрят по тексту JSON-числа до перевода во `float64`: само `float64` не отличает `1.2` от `1.20` и не хранит ровно два знака.

400 не сохраняется. `IdempotencyModel` нет. Идемпотентность — это уже записанный `TransferModel` с тем же `TransferID`.

Если строка есть и `from_account_id`, `to_account_id`, `amount` те же, `PostTransfer` возвращает записанный ответ и балансы не трогает. Исполненный перевод — 200. Нехватка средств — 422 `insufficient funds`, даже если деньги позже появились. Если строка есть, а тело другое — 409 `idempotency key reused with different payload`.

Если строки нет:

1. Оба аккаунта существуют. Иначе 422 `account not found`, даже когда счета совпали. Строка не пишется. `GetAccount` на тот же отсутствующий аккаунт отвечает 404 с тем же `reason`.
2. Перевод самому себе: 422 `self-transfer not allowed`. Строка не пишется.
3. Одна транзакция `REPEATABLE READ`, без повтора внутри сервиса. `SELECT … FOR UPDATE` исходящего аккаунта, проверка `balance >= amount`. Нехватка — в `transfers` пишется отказ `insufficient funds`, балансы не меняются, ответ 422. Иначе в этой же транзакции: вставка исполненного перевода с этим `TransferID`, исходящий `balance` уменьшается на `amount`, входящий увеличивается как `balance = balance + amount`, ответ 200.

`could not serialize access due to concurrent update` и `deadlock detected` откатывают транзакцию. Строка этой попытки не остаётся. Оба случая — один ответ 409 `transfer transaction rolled back, retry the request`. Сервис транзакцию заново не открывает, повтор решает клиент. Уникальный `TransferID` — отдельный 409 `transfer_id already exists`. Следующий запрос клиента с тем же ключом либо увидит уже записанный исход, либо проведёт перевод заново. Нет соединения с Postgres — 500 `postgres connection failed`.

Входящий `balance` не записывают абсолютным числом из чтения без его блокировки. Пустой `Reason` значит, что отказа нет. `GetTransfer` читает ту же строку `TransferModel`. Пока её нет, ответ 404.

Начальные `acc_1` … `acc_10` с балансом 1000 создаются, только если флаг конфигурации включён. Уже существующие балансы при старте не перезаписываются.

## Ручки

Подробно, с примерами ответов и кодами ошибок — в Swagger: `docs/swagger.yaml`.

Интерфейс: `http://localhost:3000/swagger/index.html`. Спека: `http://localhost:3000/swagger/doc.json`. Её соберут комментарии swag на этих методах. Текст комментариев совпадает со спекой.

| Хендлер | Метод и путь | Кратко |
|---|---|---|
| `PostTransfer` | `POST /api/v1/transfers` | Переводит монеты. Повтор ключа возвращает уже записанный перевод или уже записанную нехватку средств. Откат транзакции из-за другой транзакции — 409 `transfer transaction rolled back, retry the request`. Занятый `transfer_id` — отдельный 409. Нет соединения с Postgres — 500. |
| `GetAccount` | `GET /api/v1/accounts/{id}` | Возвращает `account_id` и баланс. Нет аккаунта — 404 `account not found`, тот же `reason`, что 422 у `PostTransfer`. |
| `GetTransfer` | `GET /api/v1/transfers/{idempotency_key}` | Возвращает записанный `TransferModel`: исполненный перевод или нехватку средств. Для нехватки HTTP 200, `status=rejected`. Нет строки — 404. |

## Транспортные сообщения

В запросе поля — указатели: нет поля и пустая строка различаются. `Amount` — `*float64`, в JSON число.

`TransferRequest`: `TransferID` с именем в JSON `idempotency_key`, `FromAccountID`, `ToAccountID`, `Amount`.

`TransferResponse` отдаёт и `PostTransfer` при 200, и `GetTransfer` при 200: `transfer_id`, `idempotency_key`, `status`, `status_code`, `from_account_id`, `to_account_id`, `amount`. `amount` — число. `created_at` — указатель, в JSON поле можно не присылать и можно прислать `null`. `reason` есть только у сохранённого отказа.

`AccountResponse`: `account_id`, `balance` числом.

Ошибка — `errors.Response`: `status_code`, `status`, `reason`. `status_code` равен HTTP-коду этого ответа.

```json
{
  "status_code": 422,
  "status": "rejected",
  "reason": "insufficient funds"
}
```

## Модели

Суффикс `Model`. Пустые колонки SQL — `sql.Null*`. Критерий фильтра — указатель. В `Partial` указатель значит «поле передано».

`AccountModel`, `AccountFilter` (`AccountID`, `AccountIDs`), `AccountPartial` (`Balance`). `UpdateAccount` меняет только баланс.

`TransferModel` — записанный исход: `completed` или `insufficient funds`. `TransferID` уникален. `Reason` — `sql.NullString`, пустой у исполненного перевода. `TransferFilter` ищет по `TransferID`. `TransferPartial` пустой: строку не переписывают. `AddTransfer` в одной транзакции со списанием и зачислением, либо одной вставкой отказа без движения `balance`.

## Реестр ошибок

Тот же список в `errors.Registry` и в `docs/swagger.yaml`. Хендлер берёт запись и пишет `errors.Response`.

| Код | HTTP | status | reason |
|---|---:|---|---|
| `invalid_json` | 400 | `invalid` | `request body is not valid json` |
| `invalid_transfer_id` | 400 | `invalid` | `idempotency_key must be a uuid` |
| `invalid_from_account_id` | 400 | `invalid` | `from_account_id must be non-empty` |
| `invalid_to_account_id` | 400 | `invalid` | `to_account_id must be non-empty` |
| `invalid_amount` | 400 | `invalid` | `amount must be positive with at most 2 fraction digits` |
| `idempotency_conflict` | 409 | `conflict` | `idempotency key reused with different payload` |
| `transfer_transaction_conflict` | 409 | `conflict` | `transfer transaction rolled back, retry the request` |
| `transfer_id_conflict` | 409 | `conflict` | `transfer_id already exists` |
| `insufficient_funds` | 422 | `rejected` | `insufficient funds` |
| `self_transfer` | 422 | `rejected` | `self-transfer not allowed` |
| `account_not_found` | 422 | `rejected` | `account not found` |
| `account_missing` | 404 | `not found` | `account not found` |
| `transfer_missing` | 404 | `not found` | `transfer not found` |
| `database_unavailable` | 500 | `unavailable` | `postgres connection failed` |
