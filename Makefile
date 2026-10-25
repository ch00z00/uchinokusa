# API 定義（api/openapi.yaml）から、サーバーとアプリの型を生成する。
# 定義を変えたら `make gen-api` を実行し、生成したファイルも一緒にコミットする。

OPENAPI_TYPESCRIPT_VERSION := 7.13.0

.PHONY: gen-api gen-api-server gen-api-mobile check-api

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

# 生成したファイルが定義と一致しているか（生成し直し忘れがないか）を確かめる
check-api: gen-api
	git diff --exit-code -- server/internal/adapter/httpapi/oapi mobile/src/api
