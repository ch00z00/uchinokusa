// Package domain は、エンティティ・値オブジェクトと、外部に依存しない純粋なロジックを提供する。
package domain

import (
	"errors"
	"fmt"
)

// ドメインのエラー。各層はエラーをこれらに正規化し、HTTP のステータスへの変換は httpapi の一か所で行う。
var (
	// ErrUnauthenticated は、ID トークンがない・無効・期限切れであることを表す（要件1-3）。
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden は、世帯のメンバーだが、ロールが操作を許可していないことを表す（要件4-3）。
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound は、対象が存在しないこと、または所属していない世帯のデータであることを表す。
	// 所属していない世帯については、存在を明かさないためにこのエラーを使う（要件4-4）。
	ErrNotFound = errors.New("not found")
	// ErrInvitationInvalid は、招待が期限切れ・取り消し済み・存在しないことを表す（要件3-5）。
	// 理由を区別しないのは、総当たりする人に「このコードは存在する」と知らせないためである。
	ErrInvitationInvalid = errors.New("invitation invalid")
)

// ConflictAlreadyMember は、すでに世帯のメンバーである人が招待を受けたときの衝突のコードである（要件3-6）。
const ConflictAlreadyMember = "already_member"

// ConflictError は、状態の衝突を表す（すでに完了済み、など）。
// Code は API 定義の ErrorCode の値で、アプリはこの値で分岐する。
type ConflictError struct {
	Code string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s", e.Code)
}

// ValidationError は、入力の内容の誤りを表す。
// Fields のキーは項目名、値は理由のコード（"required"、"too_long" など）である。
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}
