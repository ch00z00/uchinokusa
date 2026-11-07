package domain

import "fmt"

// DeletedUserName は、アカウントを削除したユーザーの記録・メモに残す作業者名である（個人情報を残さないため。要件1-6）。
const DeletedUserName = "退会したユーザー"

// ConflictOwnerTransferRequired は、オーナー権限を移さないとアカウントを削除できないことを表す衝突のコードである（要件1-7）。
const ConflictOwnerTransferRequired = "owner_transfer_required"

// OwnerTransferRequiredError は、ほかにメンバーのいる世帯の唯一のオーナーなので、アカウントを削除できないことを表す（要件1-7）。
// Households は、オーナー権限を移すべき世帯（ID・名前）である。
type OwnerTransferRequiredError struct {
	Households []Household
}

func (e *OwnerTransferRequiredError) Error() string {
	return fmt.Sprintf("conflict: %s (%d households)", ConflictOwnerTransferRequired, len(e.Households))
}
