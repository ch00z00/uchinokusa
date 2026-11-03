// Package db は、マイグレーションの SQL ファイルと初期データをバイナリに埋め込んで提供する。
package db

import "embed"

// Migrations は、goose のマイグレーションの SQL ファイル（migrations/*.sql）である。
//
//go:embed migrations/*.sql
var Migrations embed.FS

// SpeciesSeed は、植物マスタの初期データ（seeds/species.yaml）である。
//
//go:embed seeds/species.yaml
var SpeciesSeed []byte
