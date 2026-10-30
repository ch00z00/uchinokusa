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
