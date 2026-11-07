package firebase

import (
	"context"
	"fmt"
	"log/slog"

	fb "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// maxTokensPerSend は、FCM に1回で送れるトークンの数である（SendEachForMulticast の上限）。
const maxTokensPerSend = 500

// Messenger は、FCM でプッシュ通知を送る。
type Messenger struct {
	client *messaging.Client
	logger *slog.Logger
}

// NewMessenger は、Messenger を作る。credentialsFile が空なら、実行環境の認証情報（Cloud Run のサービスアカウントなど）を使う。
// opts は、テストで送り先を差し替えるために使う。
func NewMessenger(ctx context.Context, projectID, credentialsFile string, logger *slog.Logger, opts ...option.ClientOption) (*Messenger, error) {
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	app, err := fb.NewApp(ctx, &fb.Config{ProjectID: projectID}, opts...)
	if err != nil {
		return nil, fmt.Errorf("firebase: new app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: messaging client: %w", err)
	}
	return &Messenger{client: client, logger: logger}, nil
}

// Send は、同じ通知をトークンごとに送り、結果をトークンの順に返す。
// トークンが無効になった（FCM の UNREGISTERED）ものは PushUnregistered、ほかの失敗は PushFailed にする。
// どのトークンにも送れなかった（FCM に届かないなど）ときは、エラーを返す。
func (m *Messenger) Send(ctx context.Context, tokens []string, msg domain.PushMessage) ([]domain.PushOutcome, error) {
	out := make([]domain.PushOutcome, 0, len(tokens))
	for start := 0; start < len(tokens); start += maxTokensPerSend {
		chunk := tokens[start:min(start+maxTokensPerSend, len(tokens))]
		res, err := m.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
			Tokens:       chunk,
			Notification: &messaging.Notification{Title: msg.Title, Body: msg.Body},
			Data:         msg.Data,
		})
		if err != nil {
			return nil, fmt.Errorf("fcm send: %w", err)
		}
		for _, r := range res.Responses {
			switch {
			case r.Success:
				out = append(out, domain.PushDelivered)
			case messaging.IsUnregistered(r.Error):
				out = append(out, domain.PushUnregistered)
			default:
				// INVALID_ARGUMENT は、トークンではなく通知の中身の誤りのこともあるので、トークンは消さない
				m.logger.WarnContext(ctx, "fcm send failed", slog.Any("error", r.Error))
				out = append(out, domain.PushFailed)
			}
		}
	}
	return out, nil
}
