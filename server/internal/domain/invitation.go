package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Invitation は、世帯への招待である。有効期限内は複数人が使える。
type Invitation struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	Role        Role // 編集者か閲覧者（オーナーは招待で付与しない）
	Code        string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// InvitationCodeAlphabet は、招待コードに使う文字である。手入力で読み間違えやすい 0・O・1・I・L を除く。
const InvitationCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const (
	invitationTokenBytes = 32 // トークンの乱数のバイト数（256ビット。設計では128ビット以上）
	invitationCodeLength = 8
)

// InvitationSecret は、招待リンクのトークンと招待コードである。
// Token は招待リンクにだけ入れ、保存しない。DB には TokenHash を保存する（要件20-3）。
type InvitationSecret struct {
	Token     string
	TokenHash []byte
	Code      string
}

// NewInvitationSecret は、暗号論的な乱数（crypto/rand）で、新しいトークンと招待コードを作る。
func NewInvitationSecret() (InvitationSecret, error) {
	b := make([]byte, invitationTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return InvitationSecret{}, fmt.Errorf("generate invitation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b) // URL にそのまま入れられる文字だけにする

	code := make([]byte, invitationCodeLength)
	max := big.NewInt(int64(len(InvitationCodeAlphabet)))
	for i := range code {
		// rand.Int は 0〜max-1 を均等に返す。乱数のバイトを max で割った余りを使うと、偏りが出る
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return InvitationSecret{}, fmt.Errorf("generate invitation code: %w", err)
		}
		code[i] = InvitationCodeAlphabet[n.Int64()]
	}

	return InvitationSecret{Token: token, TokenHash: HashInvitationToken(token), Code: string(code)}, nil
}

// HashInvitationToken は、トークンの SHA-256 を返す。招待リンクから招待を探すときにも使う。
// トークンは十分に長い乱数なので、パスワードと違い、ソルトや遅いハッシュ関数は要らない。
func HashInvitationToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// InvitationKey は、招待を探すための鍵である。Code か TokenHash のどちらか一方だけを持つ。
type InvitationKey struct {
	Code      string
	TokenHash []byte
}

// ParseInvitationKey は、招待リンクのトークンか招待コードを読む。
// 空白とハイフンを除いて大文字にした結果が、使える文字だけの8文字ならコード、そうでなければトークンとして扱う。
// トークン（43文字）とコード（8文字）は長さが違うので、取り違えることはない。
func ParseInvitationKey(s string) InvitationKey {
	code := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(s))
	if len(code) == invitationCodeLength && strings.Trim(code, InvitationCodeAlphabet) == "" {
		return InvitationKey{Code: code}
	}
	return InvitationKey{TokenHash: HashInvitationToken(s)}
}

// InvitationPreview は、参加の確認画面に表示する、招待の内容である（要件3-3・3-6）。
type InvitationPreview struct {
	HouseholdName string
	Role          Role
	ExpiresAt     time.Time
	AlreadyMember bool
}
