# 定義ファイルから、コードを生成する。生成したファイルも一緒にコミットする。
# - API 定義（api/openapi.yaml）を変えたら `make gen-api`
# - マイグレーション・クエリ（server/db/）を変えたら `make gen-db`

OPENAPI_TYPESCRIPT_VERSION := 7.13.0

.PHONY: gen-api gen-api-server gen-api-mobile gen-db check-gen

gen-api: gen-api-server gen-api-mobile

# Go のサーバー：server/internal/adapter/httpapi/oapi/oapi.gen.go
gen-api-server:
	cd server && go generate ./internal/adapter/httpapi/oapi

# モバイルアプリ：mobile/src/api/schema.d.ts（mobile/ はタスク 14.1 で作る）
gen-api-mobile:
	@if [ -d mobile ]; then \
		npx --yes openapi-typescript@$(OPENAPI_TYPESCRIPT_VERSION) api/openapi.yaml -o mobile/src/api/schema.d.ts; \
	else \
		echo "mobile/ がまだないため、アプリの型の生成を飛ばす"; \
	fi

# sqlc：server/db/queries/*.sql → server/internal/adapter/postgres/sqlcgen/
gen-db:
	cd server && go generate ./internal/adapter/postgres

# 生成したファイルが定義と一致しているか（生成し直し忘れがないか）を確かめる
check-gen: gen-api gen-db
	git diff --exit-code -- server/internal/adapter/httpapi/oapi server/internal/adapter/postgres/sqlcgen mobile/src/api
