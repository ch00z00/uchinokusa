-- name: TryAdvisoryLock :one
-- 名前のロック（advisory lock）を待たずに取る。取れたら true。接続（セッション）が持ち、外すか接続が切れるまで残る
SELECT pg_try_advisory_lock(hashtextextended(sqlc.arg(key)::text, 0))::boolean AS locked;

-- name: AdvisoryUnlock :one
SELECT pg_advisory_unlock(hashtextextended(sqlc.arg(key)::text, 0))::boolean AS unlocked;
