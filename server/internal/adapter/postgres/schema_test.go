package postgres_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"github.com/ch00z00/uchinokusa/server/db"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
)

// PostgreSQL のエラーコード（SQLSTATE）
const (
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
	checkViolation      = "23514"
)

// マイグレーションを適用し、すべて戻し、もう一度適用できること。Down の書き漏れを見つける。
func TestMigrate_UpDownUp(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dbURL := pgtest.NewEmptyURL(t)

	if err := postgres.Migrate(ctx, dbURL); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := postgres.Migrate(ctx, dbURL); err != nil {
		t.Fatalf("Migrate again (should be no-op): %v", err)
	}

	sqlDB, err := postgres.OpenSQL(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, mustSub(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("DownTo(0): %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up after down: %v", err)
	}
}

func TestPrimaryKeyIsUUIDv7(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	id, err := uuid.Parse(f.household())
	if err != nil {
		t.Fatal(err)
	}
	if id.Version() != 7 {
		t.Errorf("version = %d, want 7 (id = %s)", id.Version(), id)
	}
}

// 将来の項目を入れる attributes は、初期値が空のオブジェクトで、任意の JSON を保存できる（要件19-8）。
func TestAttributesColumns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	loc := f.location(hh)
	plant := f.plant(hh, &loc)
	f.exec(`INSERT INTO species (name_ja) VALUES ('モンステラ')`)

	for _, table := range []string{"households", "locations", "plants", "species"} {
		var attrs string
		f.queryRow(`SELECT attributes::text FROM ` + table + ` LIMIT 1`).scan(&attrs)
		if attrs != "{}" {
			t.Errorf("%s.attributes = %s, want {}", table, attrs)
		}
	}

	f.exec(`UPDATE plants SET attributes = '{"potSizeCm": 18, "tags": ["窓際"]}' WHERE id = $1`, plant)
	var pot int
	f.queryRow(`SELECT (attributes->>'potSizeCm')::int FROM plants WHERE id = $1`, plant).scan(&pot)
	if pot != 18 {
		t.Errorf("potSizeCm = %d, want 18", pot)
	}
}

func TestUniqueConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// insert は、同じ行を2回入れる操作を返す。2回目がユニーク制約の違反になることを確かめる
		insert func(f *fixture) func() error
	}{
		{
			name: "同じ予定の回への完了は1件まで（要件11-4）",
			insert: func(f *fixture) func() error {
				hh := f.household()
				task := f.task(hh, f.plant(hh, nil))
				return func() error { return f.tryRecord(task, "completed", 1, false) }
			},
		},
		{
			name: "招待のトークンのハッシュはユニーク",
			insert: func(f *fixture) func() error {
				hh := f.household()
				n := 0
				return func() error {
					n++
					// コードは毎回変え、トークンのハッシュだけを同じにする
					return f.try(`INSERT INTO invitations (household_id, role, token_hash, code) VALUES ($1, 'editor', 'same-hash', $2)`,
						hh, []string{"AAAA2222", "BBBB3333"}[n-1])
				}
			},
		},
		{
			name: "招待コードはユニーク",
			insert: func(f *fixture) func() error {
				hh := f.household()
				n := 0
				return func() error {
					n++
					return f.try(`INSERT INTO invitations (household_id, role, token_hash, code) VALUES ($1, 'viewer', $2, 'SAME2222')`,
						hh, []string{"hash-1", "hash-2"}[n-1])
				}
			},
		},
		{
			name: "リマインダーはメンバーごとに1日1回（要件15-2）",
			insert: func(f *fixture) func() error {
				hh := f.household()
				m := f.membership(hh, f.user(), "viewer")
				return func() error {
					return f.try(`INSERT INTO reminder_dispatches (membership_id, local_date) VALUES ($1, '2026-10-26')`, m)
				}
			},
		},
		{
			name: "提案は世帯ごとに dedupe_key が重複しない（要件17-6）",
			insert: func(f *fixture) func() error {
				hh := f.household()
				plant := f.plant(hh, nil)
				return func() error {
					return f.try(`INSERT INTO suggestions (household_id, plant_id, provider_id, kind, action, message_key, reason_key, valid_from, dedupe_key)
						VALUES ($1, $2::uuid, 'repotting.v1', 'do', 'create_task', 'm', 'r', '2026-10-26', 'repotting:initial:' || $2::text)`, hh, plant)
				}
			},
		},
		{
			name: "同じユーザーは同じ世帯に1回だけ所属する",
			insert: func(f *fixture) func() error {
				hh, u := f.household(), f.user()
				return func() error {
					return f.try(`INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, 'editor')`, hh, u)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)

			insert := tt.insert(f)
			if err := insert(); err != nil {
				t.Fatalf("first insert: %v", err)
			}
			assertSQLState(t, insert(), uniqueViolation)
		})
	}
}

// 追加の記録（is_extra）とスキップは、同じ回に何件あってもよい。
func TestCompletionUniqueness_AllowsExtraAndSkips(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	task := f.task(hh, f.plant(hh, nil))

	for i, r := range []struct {
		outcome string
		extra   bool
	}{
		{"completed", false},
		{"completed", true},
		{"completed", true},
		{"skipped", false},
		{"skipped", false},
	} {
		if err := f.tryRecord(task, r.outcome, 1, r.extra); err != nil {
			t.Errorf("record %d (%s, extra=%v): %v", i, r.outcome, r.extra, err)
		}
	}
	// 次の回の完了は、別の回なので入れられる
	if err := f.tryRecord(task, "completed", 2, false); err != nil {
		t.Errorf("completion for the next cycle: %v", err)
	}
}

// 世帯の削除で、配下のデータがすべて消える（要件2-6）。
func TestDeleteHousehold_Cascades(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	other := f.household() // 別の世帯のデータは消えない
	f.seedHousehold(hh)
	f.seedHousehold(other)

	f.exec(`DELETE FROM households WHERE id = $1`, hh)

	for _, table := range householdTables {
		if n := f.count(`SELECT count(*) FROM `+table+` WHERE household_id = $1`, hh); n != 0 {
			t.Errorf("%s: %d rows remain, want 0", table, n)
		}
		if n := f.count(`SELECT count(*) FROM `+table+` WHERE household_id = $1`, other); n == 0 {
			t.Errorf("%s: rows of another household were deleted", table)
		}
	}
	if n := f.count(`SELECT count(*) FROM reminder_dispatches`); n != 1 {
		t.Errorf("reminder_dispatches = %d, want 1 (only the other household's)", n)
	}
}

// 植物の削除で、タスク・記録・メモ・提案が消える（要件5-7）。
func TestDeletePlant_Cascades(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	plant := f.plant(hh, nil)
	keep := f.plant(hh, nil)
	for _, p := range []string{plant, keep} {
		task := f.task(hh, p)
		f.record(task, "completed", 1, false)
		f.note(hh, p)
		f.suggestion(hh, p)
	}

	f.exec(`DELETE FROM plants WHERE id = $1`, plant)

	for _, table := range []string{"care_tasks", "care_records", "notes", "suggestions"} {
		if n := f.count(`SELECT count(*) FROM `+table+` WHERE plant_id = $1`, plant); n != 0 {
			t.Errorf("%s: %d rows remain, want 0", table, n)
		}
		if n := f.count(`SELECT count(*) FROM `+table+` WHERE plant_id = $1`, keep); n != 1 {
			t.Errorf("%s: rows of another plant = %d, want 1", table, n)
		}
	}
}

// 置き場所の削除で、植物は消えずに「置き場所未設定」になる（要件7-4）。
func TestDeleteLocation_UnsetsPlantLocation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	loc := f.location(hh)
	plant := f.plant(hh, &loc)

	f.exec(`DELETE FROM locations WHERE id = $1`, loc)

	var locationID *string
	var householdID string
	f.queryRow(`SELECT location_id, household_id FROM plants WHERE id = $1`, plant).scan(&locationID, &householdID)
	if locationID != nil {
		t.Errorf("location_id = %v, want NULL", *locationID)
	}
	if householdID != hh {
		t.Errorf("household_id = %s, want %s (must be kept)", householdID, hh)
	}
}

// タスクを削除しても、記録は履歴として残る（要件8-10）。
func TestDeleteTask_KeepsRecords(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	task := f.task(hh, f.plant(hh, nil))
	rec := f.record(task, "completed", 1, false)

	// 通常の削除は deleted_at を立てる論理削除
	f.exec(`UPDATE care_tasks SET deleted_at = now() WHERE id = $1`, task)
	if n := f.count(`SELECT count(*) FROM care_records WHERE task_id = $1`, task); n != 1 {
		t.Errorf("records after soft delete = %d, want 1", n)
	}

	// 行が物理的に消えた場合も、記録は残り、タスクへの参照だけが外れる
	f.exec(`DELETE FROM care_tasks WHERE id = $1`, task)
	var taskID *string
	var kind string
	f.queryRow(`SELECT task_id, task_kind FROM care_records WHERE id = $1`, rec).scan(&taskID, &kind)
	if taskID != nil {
		t.Errorf("task_id = %v, want NULL", *taskID)
	}
	if kind != "watering" {
		t.Errorf("task_kind = %q, want watering (kept for history)", kind)
	}
}

// 別の世帯の植物・タスク・置き場所を参照する行は作れない（設計の不変条件、要件20-2）。
func TestCrossHouseholdReferences_AreRejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh, other := f.household(), f.household()
	otherLoc := f.location(other)
	otherPlant := f.plant(other, nil)
	otherTask := f.task(other, otherPlant)

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{"植物 → 別の世帯の置き場所",
			`INSERT INTO plants (household_id, name, location_id) VALUES ($1, 'ポトス', $2)`, []any{hh, otherLoc}},
		{"タスク → 別の世帯の植物",
			`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on) VALUES ($1, $2, 'watering', 7, 'day', '2026-10-26')`,
			[]any{hh, otherPlant}},
		{"記録 → 別の世帯のタスク",
			`INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_display_name, cycle_seq)
			 VALUES ($1, $2, $3, 'watering', 'completed', '2026-10-26', 'A', 1)`,
			[]any{hh, f.plant(hh, nil), otherTask}},
		{"メモ → 別の世帯の植物",
			`INSERT INTO notes (household_id, plant_id, author_display_name, body) VALUES ($1, $2, 'A', 'メモ')`, []any{hh, otherPlant}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSQLState(t, f.try(tt.sql, tt.args...), foreignKeyViolation)
		})
	}
}

func TestCheckConstraints(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	plant := f.plant(hh, nil)

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{"「その他」のタスクは作業名が必須（要件8-3）",
			`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on) VALUES ($1, $2, 'other', 7, 'day', '2026-10-26')`,
			[]any{hh, plant}},
		{"有効なタスクは次回予定が必須",
			`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit) VALUES ($1, $2, 'watering', 7, 'day')`,
			[]any{hh, plant}},
		{"周期は1以上",
			`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on) VALUES ($1, $2, 'watering', 0, 'day', '2026-10-26')`,
			[]any{hh, plant}},
		{"知らないロール",
			`INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, 'admin')`, []any{hh, f.user()}},
		{"招待でオーナーは指定できない",
			`INSERT INTO invitations (household_id, role, token_hash, code) VALUES ($1, 'owner', 'h', 'CODE2222')`, []any{hh}},
		{"メモは2000文字まで",
			`INSERT INTO notes (household_id, plant_id, author_display_name, body) VALUES ($1, $2, 'A', repeat('あ', 2001))`, []any{hh, plant}},
		{"空のメモは作れない",
			`INSERT INTO notes (household_id, plant_id, author_display_name, body) VALUES ($1, $2, 'A', '')`, []any{hh, plant}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSQLState(t, f.try(tt.sql, tt.args...), checkViolation)
		})
	}

	// 一時停止中のタスクは、次回予定がなくてもよい
	if err := f.try(`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, status) VALUES ($1, $2, 'watering', 7, 'day', 'paused')`, hh, plant); err != nil {
		t.Errorf("paused task without next_due_on: %v", err)
	}
}

func TestUpdatedAtTrigger(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hh := f.household()
	// 作成時刻を過去にずらしてから更新し、updated_at が新しくなることを確かめる
	f.exec(`UPDATE households SET updated_at = '2000-01-01' WHERE id = $1`, hh)
	f.exec(`UPDATE households SET name = '実家' WHERE id = $1`, hh)

	var updatedRecently bool
	f.queryRow(`SELECT updated_at > now() - interval '1 minute' FROM households WHERE id = $1`, hh).scan(&updatedRecently)
	if !updatedRecently {
		t.Error("updated_at was not refreshed on update")
	}
}

// --- テスト用のデータを作る補助 ---

// householdTables は、household_id を持つテーブルである。
var householdTables = []string{
	"memberships", "invitations", "locations", "plants", "care_tasks",
	"care_records", "notes", "suggestions", "notification_outbox",
}

type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{t: t, ctx: t.Context(), pool: pgtest.New(t)}
}

func (f *fixture) try(sql string, args ...any) error {
	_, err := f.pool.Exec(f.ctx, sql, args...)
	return err
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if err := f.try(sql, args...); err != nil {
		f.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *fixture) id(sql string, args ...any) string {
	f.t.Helper()
	var id string
	f.queryRow(sql+` RETURNING id::text`, args...).scan(&id)
	return id
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	f.queryRow(sql, args...).scan(&n)
	return n
}

type row struct {
	t   *testing.T
	row pgx.Row
}

func (f *fixture) queryRow(sql string, args ...any) row {
	return row{t: f.t, row: f.pool.QueryRow(f.ctx, sql, args...)}
}

func (r row) scan(dest ...any) {
	r.t.Helper()
	if err := r.row.Scan(dest...); err != nil {
		r.t.Fatalf("scan: %v", err)
	}
}

func (f *fixture) user() string {
	f.t.Helper()
	return f.id(`INSERT INTO users (firebase_uid, display_name, auth_provider) VALUES (gen_random_uuid()::text, 'テスト', 'google.com')`)
}

func (f *fixture) household() string {
	f.t.Helper()
	return f.id(`INSERT INTO households (name) VALUES ('自宅')`)
}

func (f *fixture) membership(hh, user, role string) string {
	f.t.Helper()
	return f.id(`INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, $3)`, hh, user, role)
}

func (f *fixture) location(hh string) string {
	f.t.Helper()
	return f.id(`INSERT INTO locations (household_id, name) VALUES ($1, 'リビング')`, hh)
}

func (f *fixture) plant(hh string, location *string) string {
	f.t.Helper()
	return f.id(`INSERT INTO plants (household_id, name, location_id) VALUES ($1, 'モンステラ', $2)`, hh, location)
}

func (f *fixture) task(hh, plant string) string {
	f.t.Helper()
	return f.id(`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on)
		VALUES ($1, $2, 'watering', 7, 'day', '2026-10-26')`, hh, plant)
}

func (f *fixture) tryRecord(task, outcome string, cycle int, extra bool) error {
	return f.try(`INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_display_name, cycle_seq, is_extra)
		SELECT household_id, plant_id, id, kind, $2, '2026-10-26', 'テスト', $3, $4 FROM care_tasks WHERE id = $1`,
		task, outcome, cycle, extra)
}

func (f *fixture) record(task, outcome string, cycle int, extra bool) string {
	f.t.Helper()
	return f.id(`INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_display_name, cycle_seq, is_extra)
		SELECT household_id, plant_id, id, kind, $2, '2026-10-26', 'テスト', $3, $4 FROM care_tasks WHERE id = $1`,
		task, outcome, cycle, extra)
}

func (f *fixture) note(hh, plant string) string {
	f.t.Helper()
	return f.id(`INSERT INTO notes (household_id, plant_id, author_display_name, body) VALUES ($1, $2, 'テスト', '葉が増えた')`, hh, plant)
}

func (f *fixture) suggestion(hh, plant string) string {
	f.t.Helper()
	return f.id(`INSERT INTO suggestions (household_id, plant_id, provider_id, kind, action, message_key, reason_key, valid_from, dedupe_key)
		VALUES ($1, $2::uuid, 'repotting.v1', 'do', 'create_task', 'm', 'r', '2026-10-26', 'repotting:initial:' || $2::text)`, hh, plant)
}

// seedHousehold は、householdTables のすべてに1行以上のデータを入れる。
func (f *fixture) seedHousehold(hh string) {
	f.t.Helper()
	user := f.user()
	m := f.membership(hh, user, "owner")
	f.exec(`INSERT INTO invitations (household_id, role, token_hash, code) VALUES ($1, 'editor', gen_random_uuid()::text::bytea, substr(md5(random()::text), 1, 8))`, hh)
	loc := f.location(hh)
	plant := f.plant(hh, &loc)
	task := f.task(hh, plant)
	rec := f.record(task, "completed", 1, false)
	f.note(hh, plant)
	f.suggestion(hh, plant)
	f.exec(`INSERT INTO notification_outbox (household_id, kind, actor_user_id, record_id) VALUES ($1, 'completion', $2, $3)`, hh, user, rec)
	f.exec(`INSERT INTO reminder_dispatches (membership_id, local_date) VALUES ($1, '2026-10-26')`, m)
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want SQLSTATE %s", err, want)
	}
	if pgErr.Code != want {
		t.Errorf("SQLSTATE = %s (%s), want %s", pgErr.Code, pgErr.Message, want)
	}
}

func mustSub(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}
