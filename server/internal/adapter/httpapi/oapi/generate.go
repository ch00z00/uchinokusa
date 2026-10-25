// Package oapi は、api/openapi.yaml から oapi-codegen で生成した型とサーバーのインターフェースを持つ。
// oapi.gen.go は手で編集しない。定義を変えたら `go generate ./...` で生成し直す。
package oapi

//go:generate go tool oapi-codegen -config config.yaml ../../../../../api/openapi.yaml
