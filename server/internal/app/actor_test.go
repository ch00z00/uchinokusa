package app_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
)

func TestActor(t *testing.T) {
	t.Parallel()
	if got := app.ActorFrom(context.Background()); got != uuid.Nil {
		t.Errorf("without actor = %v, want uuid.Nil", got)
	}
	id := uuid.Must(uuid.NewV7())
	if got := app.ActorFrom(app.WithActor(context.Background(), id)); got != id {
		t.Errorf("actor = %v, want %v", got, id)
	}
}
