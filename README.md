### Hexlet tests and linter status:
[![Actions Status](https://github.com/slamix/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/slamix/go-project-278/actions)

### Link-shortener-service

Deploy: https://link-sortener-service.onrender.com/#/links

## Setup

Create a `.env` file for local development:

```env
DATABASE_URL=postgres://user:password@localhost:5432/link_shortener?sslmode=disable
MIGRATE_DATABASE_URL=postgres://user:password@localhost:5432/link_shortener?sslmode=disable
PORT=8080
SHORT_URL_BASE=http://localhost:8080/r
SENTRY_DSN=
```

`SHORT_URL_BASE` is used to build public short links in API responses. In production it should point to the deployed redirect endpoint, for example:

```env
SHORT_URL_BASE=https://link-sortener-service.onrender.com/r
```

## Commands

```bash
make start
make test
make lint
make build
```
