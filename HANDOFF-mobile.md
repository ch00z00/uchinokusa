# 引き継ぎメモ（アプリ担当）

アプリ（`mobile/`）の担当のエージェントは、作業を再開するときにまずこのファイルを読むこと。バックエンドの担当は `HANDOFF.md` を読む（このファイルの「バックエンドへの依頼」だけは見る）。

## 現在の状況（2026-11-08 時点）
- 仕様（`.kiro/specs/household-plant-care-mvp/`）の要件・設計・タスクは承認済み。アプリのタスクは 14〜20
- アプリのタスクは未着手。`mobile/` はまだない
- アプリが使う API は、すべてできている（バックエンドのタスク 1〜13 は完了）

## 次にやること
```
/kiro:spec-impl household-plant-care-mvp 14.1
```
- 始める前に、ユーザーと決める：デザイントークン（`docs/design/foundations.md` の値を `mobile/src/theme/` に写す作業）を `tasks.md` に足すか（14.1 に含める、または 14.3 を作る）

## ユーザーと決めたこと（2026-11-08）
- **テストの進め方**：TDD にはしない。サクサク実装する
  - 毎回、型チェック（`tsc`）・lint・formatter が通ることを確かめる
  - ロジックとして切り出せていてテストできるもの（util、フォームのスキーマなど）は、Jest と Testing Library でユニットテストのファイルを作る
- **画面の確かめ方**：スマホの画面を、実際にブラウザで確かめながら進めることを大事にする
  - Expo の web（`npx expo start --web`）をスマホの幅で表示して確かめる（Claude は Chrome の操作ツールで見られる）
  - ブラウザでは `@react-native-firebase` が動かないので、開発では Firebase の JS SDK で Auth Emulator にサインインする（14.1。サーバーは Emulator のトークンを受け付ける）
  - iOS の標準部品は、ブラウザでは見た目が変わる。形と流れはブラウザで試し、最後の見た目は iOS シミュレーターで確かめる
- **デザインの振り返り（`/retro`）**：デザインの作業の区切り（1画面が終わったときなど）で `/retro` を呼ぶ。試行錯誤から好みを抜き出して `docs/design/preferences.md` を育て、デザインの作業の前に読む（`.claude/skills/retro/SKILL.md`）

## リリースの前に必ずやること
- 本番のビルド（iOS / Android）の認証を、ブラウザの開発用の Firebase の JS SDK から、スマホ専用の `@react-native-firebase/auth` に切り替える（`tasks.md` の 14.2 に入れてある）

## バックエンドからの申し送り
- アカウントの削除（`DELETE /v1/me`、19.2 の画面）：
  - Apple でサインインしたユーザーは、削除の直前に Apple で再認証し、受け取った認可コードを `appleAuthorizationCode` で送る（ないと 422 `required`、期限切れ・使用済みだと `invalid`）
  - ほかにメンバーのいる世帯の唯一のオーナーなら、`409 owner_transfer_required` と移すべき世帯の一覧が返るので、オーナー権限の移譲へ案内する
  - 削除したユーザーの記録・メモの作業者名は「退会したユーザー」になるが、その変更のイベントは出ない。**メンバーの削除（`member` の `deleted`）のイベントを受けたら、その世帯のデータを取り直す**
- 詳しくは `api/openapi.yaml` の `DELETE /me` と `docs/decisions/0035`

## 担当の分け方
- バックエンドとアプリは、別々のエージェントが同じ作業フォルダ・同じブランチ（`main`）で並行して進める
- **アプリ担当がコミットしてよいファイル**：`mobile/`（ただし `mobile/src/api/schema.d.ts` を除く）、`HANDOFF-mobile.md`、`tasks.md` のアプリのタスク（14〜20）の行、デザインの振り返りのファイル（`.claude/skills/retro/`、`docs/design/preferences.md`、`docs/design/retro/`）
- コミットは、自分のファイルのパスを指定して行う。`git add -A` や `git add .` は使わない（もう片方の作りかけの変更が入るため）
  ```
  git add <自分のファイル>
  git commit -m "..." -- <自分のファイル>
  ```
- `docs/decisions/` に記録を足すときは、作る直前にフォルダの最新の番号を確かめる（もう片方も同じ番号で作ることがあるため）

## API の決まり
- **API の仕様は `api/openapi.yaml` が唯一の正（SSoT）**。アプリ担当は変えない
- 型は `make gen-api` で `mobile/src/api/schema.d.ts` に生成する。生成したファイルは、`openapi.yaml` を変えたバックエンド担当が一緒にコミットする
- 実装していて API に問題があったら（足りない項目、使いにくい形など）、下の「バックエンドへの依頼」に書き、ユーザーに伝える。アプリ側で回避の作り込みはしない

## バックエンドへの依頼
バックエンド担当は、対応したら「対応済み」とコミットを書き添える。

（まだなし）

## デザイン
- 画面と部品の実装では、必ず `.kiro/steering/design.md` を読んで従う
- 画面の一覧は `docs/design/screen-inventory.md`、色や文字などの値は `docs/design/foundations.md`
- 未決：「植物」タブの名前、要件 9-5（置き場所ごと・種類ごとの表示の切り替え）の扱い。16.2 までに決める

## 進め方
- 1タスクずつ進め、各タスクの冒頭で「何を・なぜ」を短く説明し、終わったら止まってユーザーの確認をもらう
- タスクが終わったら `tasks.md` のチェックボックスを `[x]` にし、コミットする
- push は大きいタスク（14, 15 など）の単位で行う
