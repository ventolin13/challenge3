export PATH := $(shell go env GOPATH)/bin:$(PATH)

.PHONY: help # show targets
help:
	@grep '^.PHONY: .* #' Makefile | sed 's/\.PHONY: \(.*\) # \(.*\)/\1 \2/' | expand -t20

.PHONY: init-mock # install mockery
init-mock:
	go install github.com/vektra/mockery/v2@v2.53.3

.PHONY: init-lint # install golangci-lint
init-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

.PHONY: migrate-up # apply schema and seed account balances
migrate-up:
	docker compose up -d --wait postgres
	docker compose run --rm --no-deps flyway

.PHONY: migrate-down # drop all tables
migrate-down:
	docker compose up -d --wait postgres
	./scripts/migrate-down.sh

.PHONY: test # run go tests
test:
	go test ./...

.PHONY: test-verbose # run go tests and print goroutine and transaction logs
test-verbose:
	go test -v ./...

.PHONY: linter # run golangci-lint
linter:
	golangci-lint run ./...

.PHONY: mocks # generate mockery mocks
mocks:
	mockery

.DEFAULT_GOAL := help
