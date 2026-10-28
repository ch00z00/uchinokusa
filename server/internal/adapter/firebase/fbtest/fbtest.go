// Package fbtest は、結合テスト用の Firebase Auth Emulator を testcontainers で用意する。
//
// Emulator は dev/firebase-emulator の Dockerfile からビルドし、テストのバイナリ（パッケージ）ごとに1つだけ起動する。
// Firebase Admin SDK は、環境変数 FIREBASE_AUTH_EMULATOR_HOST があると Emulator に接続する。
package fbtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ProjectID は、Emulator のデモプロジェクトの ID である（dev/firebase-emulator/Dockerfile と同じ）。
const ProjectID = "demo-uchinokusa"

var (
	once     sync.Once
	host     string // Emulator の host:port
	setupErr error
)

// Start は、Emulator を起動し、FIREBASE_AUTH_EMULATOR_HOST を設定する。
// Admin SDK のクライアントを作る前に呼ぶ。`go test -short` のときは、テストを飛ばす。
func Start(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("fbtest: skipping integration test in short mode")
	}
	once.Do(func() { setupErr = setup(context.Background()) })
	if setupErr != nil {
		t.Fatalf("fbtest: setup: %v", setupErr)
	}
}

func setup(ctx context.Context) error {
	ctr, err := testcontainers.Run(ctx, "",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{
			Context:   emulatorDir(),
			KeepImage: true, // firebase-tools のインストールに時間がかかるため、イメージを残して次回のテストで使い回す
		}),
		testcontainers.WithExposedPorts("9099/tcp"),
		testcontainers.WithWaitStrategy(wait.ForLog("All emulators ready")),
	)
	if err != nil {
		return fmt.Errorf("start emulator: %w", err)
	}
	endpoint, err := ctr.PortEndpoint(ctx, "9099/tcp", "")
	if err != nil {
		return err
	}
	host = endpoint
	return os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", host)
}

// emulatorDir は、リポジトリの dev/firebase-emulator の絶対パスを返す。
func emulatorDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "../../../../../dev/firebase-emulator")
}

// SignUp は、Emulator にユーザーを作り、その ID トークンと uid を返す。
// Emulator は、API キーとして任意の値を受け付ける。
func SignUp(t *testing.T, email, displayName string) (idToken, uid string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"email":             email,
		"password":          "password",
		"displayName":       displayName,
		"returnSecureToken": true,
	})
	url := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-api-key", host)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("fbtest: sign up: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		IDToken string `json:"idToken"`
		LocalID string `json:"localId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || resp.StatusCode != http.StatusOK || out.IDToken == "" {
		t.Fatalf("fbtest: sign up: status %d, err %v", resp.StatusCode, err)
	}
	return out.IDToken, out.LocalID
}
