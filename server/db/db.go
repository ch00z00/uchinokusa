// Package db は、マイグレーションの SQL ファイルをバイナリに埋め込んで提供する。
package db

import "embed"

// Migrations は、goose のマイグレーションの SQL ファイル（migrations/*.sql）である。
//
//go:embed migrations/*.sql
var Migrations embed.FS
