# challenge3

Сервис переводов монет между аккаунтами. Один процесс, REST на gin, одна база Postgres.

Основная документация: [docs/kak-est.md](docs/kak-est.md). Сопроводительное письмо: [docs/soprovoditelnoe-pismo.md](docs/soprovoditelnoe-pismo.md). Задание: [docs/Go_Code_Challenge_3.pdf](docs/Go_Code_Challenge_3.pdf).

Сервер слушает порт `3000`.

- Сервис: http://localhost:3000/
- Swagger: http://localhost:3000/swagger/index.html
- Спека: http://localhost:3000/swagger/doc.json

Три ручки: `POST /api/v1/transfers`, `GET /api/v1/accounts/{id}`, `GET /api/v1/transfers/{idempotency_key}`.

Начальные `acc_1` … `acc_10` с `balance = 1000.00` создаёт миграция `V1_003__seed_accounts.sql`. Повторный старт уже существующие `balance` не перезаписывает.

## Запуск

`docker-compose.yml` поднимает Postgres и Flyway. Сеть `challenge3`, база `challenge` на порту `5432`. Образ сервиса собирает `Dockerfile`, контейнер называется `challenge3-app` и слушает порт `3000`.

На хосте: `make migrate-up`, затем `make run`. Окружение `deployments/local.env`, база `127.0.0.1:5432`.

В контейнере: `make compose-up`, затем `make start-app`. Окружение `deployments/app.env`, база `postgres:5432`. Продовый контейнер: `make start-prod`, окружение `deployments/prod.env`, `SEED_ACCOUNTS=false`.

`make app-down` удаляет контейнер сервиса. `make compose-down` удаляет Postgres и Flyway. Тома у базы нет, данные уходят вместе с контейнером.

## Команды

`make` без цели выполняет `make help`. Ниже все цели Makefile, в том же порядке и с теми же командами.

| Команда | Команды цели |
|---|---|
| `make help` | Печатает этот список целей |
| `make init-mock` | `go install github.com/vektra/mockery/v2@v2.53.3` |
| `make init-lint` | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` |
| `make migrate-up` | `docker compose up -d --wait postgres`, затем `docker compose run --rm --no-deps flyway` |
| `make migrate-down` | `docker compose up -d --wait postgres`, затем `./scripts/migrate-down.sh` |
| `make compose-up` | `docker compose up` |
| `make compose-down` | `docker compose down` |
| `make run` | `env $(grep -v '^#' deployments/local.env \| xargs) go run ./cmd/challenge3` |
| `make start-app` | `docker build -t challenge3 .`, затем `docker rm -f challenge3-app`, затем `docker run --name challenge3-app --network challenge3 -p 3000:3000 --env-file deployments/app.env challenge3` |
| `make start-prod` | `docker build -t challenge3 .`, затем `docker rm -f challenge3-app`, затем `docker run --name challenge3-app --network challenge3 -p 3000:3000 --env-file deployments/prod.env challenge3` |
| `make app-down` | `docker rm -f challenge3-app` |
| `make test` | `go test ./...` |
| `make test-verbose` | `go test -v ./...` |
| `make linter` | `golangci-lint run ./...` |
| `make mocks` | `mockery` по `.mockery.yaml` из `$(go env GOPATH)/bin` |

`migrate-up` и `migrate-down` работают с базой `challenge` на порту `5432`. `make test` открывает свою базу `challenge_test` на порту `55432`. Журнал горутин и транзакций печатает `make test-verbose`.
