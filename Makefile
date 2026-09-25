# Comandos del monorepo. `make help` lista todo.
SHELL := /bin/bash
GO ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
GOPKGS := ./packages/... ./tools/... ./apps/...

.DEFAULT_GOAL := help

help: ## Muestra esta ayuda
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

hooks: ## Activa los ganchos de Git del repo (formato, secretos, Conventional Commits)
	git config core.hooksPath tools/githooks
	@echo "Ganchos activados desde tools/githooks"

dev: ## Levanta el entorno local completo (Postgres, S3, Mailpit, toxiproxy, impresoras, SRI)
	docker compose up -d --build
	@echo ""
	@echo "  Impresoras simuladas  http://localhost:8090  (TCP 9100 cocina · 9101 bar · 9102 caja)"
	@echo "  Stub del SRI          http://localhost:8091"
	@echo "  Correo (Mailpit)      http://localhost:8025"
	@echo "  PostgreSQL            localhost:$${PG_PORT:-5442}  (restpos_owner / restpos_app)"
	@echo "  S3 (VersityGW)        http://localhost:$${S3_PORT:-7171}"

down: ## Detiene el entorno local (conserva los datos)
	docker compose down

reset: ## Borra los datos locales y vuelve a levantar el entorno
	docker compose down -v
	$(MAKE) dev

logs: ## Sigue los logs del entorno local
	docker compose logs -f --tail=50

TEST_DATABASE_URL ?= postgres://restpos_owner:restpos_dev@localhost:$${PG_PORT:-5442}/restpos
# Azurite con su clave pública de emulador (no es un secreto).
TEST_AZURE_STORAGE ?= DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:$${AZURITE_PORT:-10010}/devstoreaccount1;

test: ## Pruebas Go con detector de carreras (las de Postgres corren si `make dev` está arriba)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_AZURE_STORAGE="$(TEST_AZURE_STORAGE)" $(GO) test -race -count=1 $(GOPKGS)

test-long: ## Pruebas property-based largas (QA-04: 10⁶ claves de acceso)
	$(GO) test -count=1 ./packages/go/sri -run Property -rapid.checks=1000000

cover: ## Cobertura de los dominios críticos (RNF-50: ≥ 80 %)
	$(GO) test -coverprofile=coverage.out ./packages/go/...
	$(GO) tool cover -func=coverage.out | tail -1

fmt: ## Formatea el código Go
	$(GO) fmt $(GOPKGS)

lint: check-float ## Revisión estática (go vet + golangci-lint si está instalado)
	$(GO) vet $(GOPKGS)
	@if command -v golangci-lint >/dev/null; then golangci-lint run; else echo "golangci-lint no instalado: se ejecuta en el CI"; fi

check-float: ## Falla si aparece float en código de dominio (regla: dinero siempre decimal)
	@if grep -rnE '\bfloat(32|64)\b' --include='*.go' packages apps | grep -v '_test.go'; then \
		echo "✗ Se encontró float en código de dominio. Usa money.Money o decimal."; exit 1; \
	else echo "✓ Sin float en código de dominio"; fi

golden: ## Regenera los tickets golden de ESC/POS (revisa el diff antes de confirmar)
	$(GO) test ./packages/go/escpos -update

printer-sim: ## Corre el simulador de impresoras sin Docker
	$(GO) run ./tools/printer-sim

sri-stub: ## Corre el stub del SRI sin Docker
	$(GO) run ./tools/sri-stub

tokens: ## Regenera los tokens de diseño (CSS, TS, Tailwind, Dart) y verifica contraste AA
	$(GO) run ./tools/design-tokens

tokens-check: ## Falla si los tokens generados están desactualizados o el contraste no cumple AA
	$(GO) run ./tools/design-tokens -check

ui-test: ## Tipos y pruebas del paquete @restpos/ui
	cd packages/ts/ui && npx -y -p typescript@5.9 tsc --noEmit -p . && node --experimental-strip-types --test src/*.test.ts

ui-docs: ## Sirve la guía de estilo viva en http://localhost:8095/docs/
	@echo "Guía de estilo: http://localhost:8095/docs/"
	cd packages/ts/ui && python3 -m http.server 8095 --bind 127.0.0.1

flutter-ui: ## Analiza y prueba el paquete Flutter restpos_ui con Docker (no requiere Flutter instalado)
	docker run --rm -v "$$PWD/packages/dart/restpos_ui":/pkg -w /pkg ghcr.io/cirruslabs/flutter:stable sh -c "flutter pub get && flutter analyze && flutter test; s=$$?; chown -R $$(id -u):$$(id -g) /pkg; exit $$s"

api: ## Corre la API en :8080 (migra y siembra la galería en local; requiere `make dev`)
	$(GO) run ./apps/cloud-api/cmd/cloud-api serve

demo: ## Crea el restaurante de demostración «Cevichería Don Pepe» con fotos (demo@donpepe.ec / DonPepe2026)
	$(GO) run ./apps/cloud-api/cmd/cloud-api demo

nodo: ## Corre el Nodo Local en primer plano en http://localhost:7080 (datos en .nodo-data/)
	RESTPOS_DATA=$(CURDIR)/.nodo-data RESTPOS_HTTP=127.0.0.1:7080 $(GO) run ./apps/edge-node/cmd/restpos-nodo run

nodo-win: ## Compila el Nodo Local para Windows x64 en dist/restpos-nodo.exe
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o dist/restpos-nodo.exe ./apps/edge-node/cmd/restpos-nodo

bo: ## Corre el backoffice en http://localhost:5173 (con `make api` en otra terminal)
	npm install --no-audit --no-fund && npm run dev -w @restpos/backoffice-web

bo-test: ## Tipos, pruebas y build del backoffice
	npm run typecheck -w @restpos/backoffice-web && npm test -w @restpos/backoffice-web && npm run build -w @restpos/backoffice-web

backlog: ## Regenera docs/12-backlog-tickets.md desde el JSON
	python3 documentacion-proyecto/docs/backlog/generar.py

.PHONY: help hooks dev down reset logs test test-long cover fmt lint check-float golden printer-sim sri-stub tokens tokens-check ui-test ui-docs flutter-ui api demo bo bo-test backlog
