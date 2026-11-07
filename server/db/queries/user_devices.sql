-- name: UpsertUserDevice :one
-- 同じ fcm_token の行があれば、ユーザー・OS・タイムゾーンを上書きする。
-- 別のユーザーの行だった場合も上書きし、新しいユーザーに付け替える（1台の端末でユーザーを切り替えた場合）
INSERT INTO user_devices (user_id, fcm_token, platform, time_zone)
VALUES ($1, $2, $3, $4)
ON CONFLICT (fcm_token) DO UPDATE
SET user_id      = EXCLUDED.user_id,
    platform     = EXCLUDED.platform,
    time_zone    = EXCLUDED.time_zone,
    last_seen_at = now()
RETURNING *;

-- name: DeleteUserDevice :exec
-- 自分の端末だけを消す（user_id の条件で、ほかのユーザーの端末を消せないようにする）
DELETE FROM user_devices
WHERE id = $1 AND user_id = $2;

-- name: ListUserDeviceTokens :many
-- ユーザーたちの端末の FCM トークン（プッシュ通知の宛先）
SELECT fcm_token FROM user_devices
WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY fcm_token;

-- name: DeleteUserDevicesByToken :exec
-- FCM が無効と答えたトークンの端末を消す（どのユーザーの端末でも消す）
DELETE FROM user_devices
WHERE fcm_token = ANY(sqlc.arg(tokens)::text[]);
