package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// DeviceTokenRepository は、プッシュ通知の宛先（端末の FCM トークン）の保存先である（実装は adapter/postgres）。
type DeviceTokenRepository interface {
	// TokensOf は、ユーザーたちの端末のトークンを返す。
	TokensOf(ctx context.Context, userIDs []uuid.UUID) ([]string, error)
	// DeleteTokens は、トークンの端末を消す。
	DeleteTokens(ctx context.Context, tokens []string) error
}

// PushSender は、プッシュ通知を送る（実装は adapter/firebase の Messenger）。結果はトークンの順に返す。
type PushSender interface {
	Send(ctx context.Context, tokens []string, msg domain.PushMessage) ([]domain.PushOutcome, error)
}

// PushService は、ユーザーの端末にプッシュ通知を送り、無効になったトークンを消す（ジョブから使う）。
type PushService struct {
	devices DeviceTokenRepository
	sender  PushSender
	logger  *slog.Logger
}

func NewPushService(devices DeviceTokenRepository, sender PushSender, logger *slog.Logger) *PushService {
	return &PushService{devices: devices, sender: sender, logger: logger}
}

// Notify は、ユーザーたちのすべての端末に通知を送る。FCM が無効と答えたトークン（アプリの削除など）は、端末ごと消す（要件15・16）。
// 端末がなければ何もしない。FCM に送れなかったときはエラーを返す（ジョブが次の実行でやり直す）。
func (s *PushService) Notify(ctx context.Context, userIDs []uuid.UUID, msg domain.PushMessage) (domain.PushReport, error) {
	var rep domain.PushReport
	if len(userIDs) == 0 {
		return rep, nil
	}
	tokens, err := s.devices.TokensOf(ctx, userIDs)
	if err != nil {
		return rep, fmt.Errorf("list device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return rep, nil
	}
	outcomes, err := s.sender.Send(ctx, tokens, msg)
	if err != nil {
		return rep, fmt.Errorf("send push: %w", err)
	}

	var unregistered []string
	for i, o := range outcomes {
		switch o {
		case domain.PushDelivered:
			rep.Delivered++
		case domain.PushUnregistered:
			unregistered = append(unregistered, tokens[i])
		default:
			rep.Failed++
		}
	}
	if len(unregistered) > 0 {
		// 消せなくても、送った結果は変わらない。次に送るときに、また無効と答えるので、そのときに消す
		if err := s.devices.DeleteTokens(ctx, unregistered); err != nil {
			s.logger.ErrorContext(ctx, "delete unregistered tokens", slog.Int("count", len(unregistered)), slog.Any("error", err))
		} else {
			rep.Removed = len(unregistered)
		}
	}
	return rep, nil
}
