---
name: challenge3
description: >-
  Boundaries for the challenge3 monolith: one process, REST only, one Postgres,
  golangci-lint 2.12. Use when writing or reviewing code in challenge3.
---

# challenge3

- Монолит: один процесс.
- Только REST.
- Одна база Postgres.

Линтер — golangci-lint 2.12, конфиг `.golangci.yml` в корне:

```bash
golangci-lint run
```
