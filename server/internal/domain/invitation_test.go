package domain_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNewInvitationSecret(t *testing.T) {
	t.Parallel()

	s, err := domain.NewInvitationSecret()
	if err != nil {
		t.Fatal(err)
	}

	// トークンは URL にそのまま入れられる形（base64url、パディングなし）で、256ビットの乱数である
	raw, err := base64.RawURLEncoding.DecodeString(s.Token)
	if err != nil {
		t.Fatalf("token %q is not base64url: %v", s.Token, err)
	}
	if len(raw) != 32 {
		t.Errorf("token has %d bytes of entropy, want 32", len(raw))
	}
	// 保存するのはトークンのハッシュだけ
	if want := sha256.Sum256([]byte(s.Token)); !bytes.Equal(s.TokenHash, want[:]) {
		t.Errorf("TokenHash is not SHA-256 of the token")
	}
	if !bytes.Equal(domain.HashInvitationToken(s.Token), s.TokenHash) {
		t.Errorf("HashInvitationToken does not match TokenHash")
	}

	// コードは8文字で、紛らわしい文字を含まない
	if len(s.Code) != 8 {
		t.Errorf("code %q has %d characters, want 8", s.Code, len(s.Code))
	}
	if strings.ContainsAny(s.Code, "0O1IL") {
		t.Errorf("code %q contains a confusing character", s.Code)
	}
}

// 生成のたびに異なる値になり、コードには使える文字がまんべんなく現れる。
func TestNewInvitationSecret_Random(t *testing.T) {
	t.Parallel()

	const n = 2000
	tokens := map[string]bool{}
	codes := map[string]bool{}
	seen := map[rune]int{}
	for range n {
		s, err := domain.NewInvitationSecret()
		if err != nil {
			t.Fatal(err)
		}
		tokens[s.Token] = true
		codes[s.Code] = true
		for _, r := range s.Code {
			seen[r]++
		}
	}
	if len(tokens) != n || len(codes) != n {
		t.Errorf("duplicates: %d unique tokens, %d unique codes out of %d", len(tokens), len(codes), n)
	}
	for _, r := range domain.InvitationCodeAlphabet {
		// 1文字あたりの期待値は 2000×8÷31 ≒ 516 回。極端な偏り（半分未満）がないことだけを確かめる
		if seen[r] < 258 {
			t.Errorf("character %q appeared only %d times", r, seen[r])
		}
	}
	for r := range seen {
		if !strings.ContainsRune(domain.InvitationCodeAlphabet, r) {
			t.Errorf("character %q is not in the alphabet", r)
		}
	}
}

func TestParseInvitationKey(t *testing.T) {
	t.Parallel()

	token := strings.Repeat("a", 43)
	tests := []struct {
		name     string
		in       string
		wantCode string // 空ならトークンとして扱う
	}{
		{name: "コード", in: "K7MP2QXA", wantCode: "K7MP2QXA"},
		{name: "小文字のコードは大文字にそろえる", in: "k7mp2qxa", wantCode: "K7MP2QXA"},
		{name: "空白とハイフンは無視する", in: " K7MP-2QXA ", wantCode: "K7MP2QXA"},
		{name: "使わない文字（O）を含む8文字は、コードではない", in: "K7MP2QXO"},
		{name: "8文字でなければ、コードではない", in: "K7MP2QX"},
		{name: "トークン", in: token},
		{name: "空", in: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ParseInvitationKey(tt.in)
			if tt.wantCode != "" {
				if got.Code != tt.wantCode || got.TokenHash != nil {
					t.Errorf("got %+v, want code %s", got, tt.wantCode)
				}
				return
			}
			// トークンは、受け取ったままの文字列のハッシュで探す
			if got.Code != "" || !bytes.Equal(got.TokenHash, domain.HashInvitationToken(tt.in)) {
				t.Errorf("got %+v, want the token hash of %q", got, tt.in)
			}
		})
	}
}
