# 引き継ぎメモ（アプリ担当）

アプリ（`mobile/`）の担当のエージェントは、作業を再開するときにまずこのファイルを読むこと。バックエンドの担当は `HANDOFF.md` を読む（このファイルの「バックエンドへの依頼」だけは見る）。

## 現在の状況（2026-11-08 時点）
- 仕様（`.kiro/specs/household-plant-care-mvp/`）の要件・設計・タスクは承認済み。アプリのタスクは 14〜20
- 14.1（Expo のプロジェクトと共通の基盤）が完了。起動・構成・API の呼び方は `mobile/README.md`
- アプリが使う API は、すべてできている（バックエンドのタスク 1〜13 は完了）。ただし、ブラウザから呼ぶには CORS が必要（下の「バックエンドへの依頼」）

## 次にやること
```
/kiro:spec-impl household-plant-care-mvp 14.2
```
- 14.3（デザイントークンをテーマに写す）を `tasks.md` に足した。14.2 と 14.3 のどちらを先にするかは、始める前にユーザーと決める
- Bundle ID / パッケージ名は、仮に `app.uchinokusa`（`mobile/app.json`）。ストアに登録する前にユーザーと決める

## 14.1 でわかったこと・決めたこと
- 画面は `mobile/src/app/` に置く（Expo SDK 58 の標準。`structure.md` の予定の `mobile/app/` とは違う。steering の更新はユーザーに確認する）
- サインインの前後の振り分けは、`(auth)` と `(app)` のグループのレイアウトで `Redirect` する。`Stack.Protected` は、試したときに更新の繰り返し（Maximum update depth）が出たので使っていない（原因の切り分けの途中で開発サーバーが古い状態だったため、`Stack.Protected` が原因だったかは確かめきれていない）
- 設定は、タブにせず、各タブのヘッダーの右のボタンから開く（画面一覧の【未決】。仕様のルーティングのとおり）
- タブバーは `expo-router/native-tabs`（OS の標準のタブバー）。アイコンは `foundations.md` の 4.3 の名前（`checklist`・`leaf` / `potted_plant`）で型チェックが通った
- ブラウザでスマホの幅の枠をアプリの中に作ると、web のヘッダーが更新を繰り返して止まる。幅は Chrome の側で合わせる
- iOS / Android のネイティブのビルドは、この Mac に Xcode・Android SDK がないため、まだ確かめていない（設定は `app.json`・`eas.json` まで）
- iOS / Android の認証も、14.2 まではブラウザと同じ Firebase の JS SDK（`src/auth/authClient.ts`）。サインインの状態は再起動で消える
- TypeScript 6 では `@types/*` を自動で読まないので、`tsconfig.json` の `types` に足す

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

### 1. CORS に対応してほしい（2026-11-08）
- ブラウザ（Expo の web、`http://localhost:8081`）から API（`http://localhost:8080`）を呼ぶと、事前確認の `OPTIONS /v1/me` が **405** になり、ブラウザがリクエストを止める（アプリには「通信できませんでした」と出る）
- ブラウザで画面を確かめながら進める方針（「ユーザーと決めたこと」）のため、REST API で CORS に対応してほしい
  - 許可するオリジンは、すでにある `ALLOWED_ORIGINS` を使う案（ローカルは `http://localhost:8081`）
  - 必要なヘッダー：`Authorization`、`Content-Type`。メソッド：`GET`・`POST`・`PUT`・`PATCH`・`DELETE`
  - 本番のアプリ（iOS / Android）はブラウザではないので CORS は関係しない。`ALLOWED_ORIGINS` が空なら、今までどおり何も許可しない、で足りるはず

### 2. `mobile/src/api/schema.d.ts` をコミットしてほしい（2026-11-08）
- `mobile/` ができたので、`make gen-api` でアプリの型が生成されるようになった。生成したファイルはバックエンド担当がコミットする決まりなので、生成してコミットしてほしい（アプリ担当は、手元で生成したものを使っていて、コミットしていない）

## デザイン
- 画面と部品の実装では、必ず `.kiro/steering/design.md` を読んで従う
- 画面の一覧は `docs/design/screen-inventory.md`、色や文字などの値は `docs/design/foundations.md`
- 未決：「植物」タブの名前、要件 9-5（置き場所ごと・種類ごとの表示の切り替え）の扱い。16.2 までに決める

## 進め方
- 1タスクずつ進め、各タスクの冒頭で「何を・なぜ」を短く説明し、終わったら止まってユーザーの確認をもらう
- タスクが終わったら `tasks.md` のチェックボックスを `[x]` にし、コミットする
- push は大きいタスク（14, 15 など）の単位で行う
