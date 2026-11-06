package app

import (
	"context"

	"github.com/google/uuid"
)

type actorKey struct{}

// WithActor は、操作した人を context に入れる。認証の middleware が入れ、変更イベント（domain.ChangeEvent）の ActorUserID に使う。
// リポジトリの引数に足さずに済むよう、context で渡す。
func WithActor(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, actorKey{}, userID)
}

// ActorFrom は、context の操作した人を返す。入っていなければ uuid.Nil を返す（ジョブなど、人の操作でない場合）。
func ActorFrom(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(actorKey{}).(uuid.UUID)
	return id
}
