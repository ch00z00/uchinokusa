-- name: BumpHouseholdVersion :one
-- 変更イベントごとに、世帯のバージョン番号を1つ進める。変更と同じトランザクションの最後に行う
-- （世帯の行のロックをいちばん後に取るので、ほかのロックとの順序の食い違いが起きない）
UPDATE households SET version = version + 1 WHERE id = $1 RETURNING version;

-- name: NotifyHouseholdChange :exec
-- 変更イベントを、LISTEN しているすべての API インスタンスに知らせる。届くのはコミットしたときだけで、ロールバックすれば届かない
SELECT pg_notify('household_changes', sqlc.arg(payload)::text);

-- name: ListUserHouseholdIDs :many
-- ユーザーが所属する世帯（表示名の変更を、各世帯のメンバーの変更として知らせるため）
SELECT household_id FROM memberships WHERE user_id = $1 ORDER BY household_id;
