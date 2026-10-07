.PHONY: docs db-up db-down db-logs test-integration

TEST_MYSQL_DSN ?= root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true
TEST_POSTGRES_DSN ?= postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable

db-up:
	docker compose up -d --wait mysql postgres

db-down:
	docker compose down

db-logs:
	docker compose logs mysql postgres

test-integration: db-up
	@TEST_MYSQL_DSN='$(TEST_MYSQL_DSN)' TEST_POSTGRES_DSN='$(TEST_POSTGRES_DSN)' go test ./... -count=1

docs:
	packages="$$(go list ./orm/... | grep -v '/internal/')"; \
	dirs="$$(go list -f '{{.Dir}}' $$packages | sed 's|$(CURDIR)|docs|')"; \
	for d in $$dirs; do mkdir -p $$d; done; \
	go run github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest --output "docs/{{.Dir}}/README.md" $$packages
