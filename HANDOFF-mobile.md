# 引き継ぎメモ（アプリ担当）

アプリ（`mobile/`）の担当のエージェントは、作業を再開するときにまずこのファイルを読むこと。バックエンドの担当は `HANDOFF.md` を読む（このファイルの「バックエンドへの依頼」だけは見る）。

## 現在の状況（2026-11-20 時点）
- 仕様（`.kiro/specs/household-plant-care-mvp/`）の要件・設計・タスクは承認済み。アプリのタスクは 14〜20
- 完了：14.1・14.2・14.3・15.1・15.2（iOS シミュレーターで確認済み）。16.1 も完了（確認済み）。起動・構成・API の呼び方は `mobile/README.md`
- 残り：14.4（本番のサインイン）と 15.3（招待用ドメイン）は、本番の Firebase・Apple Developer Program などの前提がそろってから
- アプリが使う API は、すべてできている（バックエンドのタスク 1〜13 は完了）。ただし、ブラウザから呼ぶには CORS が必要（下の「バックエンドへの依頼」）

## 次にやること
```
/kiro:spec-impl household-plant-care-mvp 16.3
```
- 16.2 は確認済み（2026-11-22）。いまは 16.3（お手入れタスクの設定）を3つに分けて進める：16.3a 詳細のタスクの一覧と作成（実装済みで確認待ち）→ 16.3b 変更・一時停止・再開・削除 → 16.3c 植え替えの予定日を避ける時期にしたときの注意
- 16.1 は確認済み（2026-11-20）
- あとで直す：世帯の設定（HH-01）に、世帯名・地域・メンバー・招待・置き場所・退出・削除が並び、項目が多くなった。ユーザーから「「世帯の設定」の中にまとめるにはデザイン的に微妙になってきた」（2026-11-20）。設定のタブの構成ごと見直す（時期はユーザーと決める）
- iOS のビルド：`npx expo run:ios --device <シミュレーターの ID> --port 8082`（Xcode 26.6、iOS 26.5 のプラットフォームは入れた。2026-11-19）。ネイティブのモジュールを足したら `npx expo prebuild --platform ios --no-install` のあとにビルドし直す
- シミュレーターは Claude からはタップできない（画面を撮る `xcrun simctl io <ID> screenshot` と、リンクを開く `xcrun simctl openurl` だけできる）。操作の確認はユーザーに頼む
- Bundle ID / パッケージ名は、仮に `app.uchinokusa`（`mobile/app.json`）。ストアに登録する前にユーザーと決める

## 14.1 でわかったこと・決めたこと
- 画面は `mobile/src/app/` に置く（Expo SDK 58 の標準。`structure.md` の予定の `mobile/app/` とは違う。steering の更新はユーザーに確認する）
- サインインの前後の振り分けは、`(auth)` と `(app)` のグループのレイアウトで `Redirect` する。`Stack.Protected` は、試したときに更新の繰り返し（Maximum update depth）が出たので使っていない。ただし、そのときは開発サーバーを `CI=1` で起動していて変更が反映されていなかったため、`Stack.Protected` が原因だったかはわからない
- ~~設定は、タブにせず、各タブのヘッダーの右のボタンから開く~~ → 15.1 の確認で、設定はタブにした（下の「15.2 で決めたこと」）
- タブバーは `expo-router/native-tabs`（OS の標準のタブバー）。アイコンは `foundations.md` の 4.3 の名前（`checklist`・`leaf` / `potted_plant`）で型チェックが通った
- ブラウザでスマホの幅の枠をアプリの中に作ると、web のヘッダーが更新を繰り返して止まる。幅は Chrome の側で合わせる
- iOS / Android のネイティブのビルドは、この Mac に Xcode・Android SDK がないため、まだ確かめていない（設定は `app.json`・`eas.json` まで）
- iOS / Android の認証も、14.4 まではブラウザと同じ Firebase の JS SDK（`src/auth/authClient.ts`）。サインインの状態は AsyncStorage に保存し、再起動後も保つ
- TypeScript 6 では `@types/*` を自動で読まないので、`tsconfig.json` の `types` に足す
- 開発サーバーを `CI=1` で起動すると、ファイルの変更が反映されない（14.1 で古い画面が出ていた原因）

## 14.2 で決めたこと
- 認証は `AuthClient`（`src/auth/types.ts`）の取り決めで、ブラウザ（`authClient.web.ts`）と iOS / Android（`authClient.ts`）に分ける。14.4 で iOS / Android の実装だけを `@react-native-firebase/auth` に差し替える
- iOS / Android は、JS SDK でポップアップを開けないため、Emulator の偽のトークンでサインインする。Apple は名前なし・Google は名前ありの決まったユーザー（`firebaseJsAuth.ts` の `emulatorCredential`）
- 表示名の画面は、初めてのサインイン（`isNewUser`）のあと、または表示名が空のときに出す。初めての印はメモリにしか持たないので、表示名の画面でアプリを閉じると、Google のユーザー（名前あり）は次の起動で画面が出ない（許容した）
- 表示名の入力チェックは、サーバー（`domain.NormalizeDisplayName`）と同じく、前後の空白を除いてコードポイントで 1〜30 文字
- サインアウトは `performSignOut`（`src/auth/signOut.ts`）：端末を通知の送信先から外す（失敗しても続ける）→ サインアウト → キャッシュと、端末に保存した値（接頭辞 `uchinokusa:`）を消す
- `getReactNativePersistence` は `firebase/auth` の型にないので、`src/auth/firebase-auth-rn.d.ts` で型を補う

## 15.1 で決めたこと
- 【未決】だった4つは、ユーザーと決めた（2026-11-17。`screen-inventory.md` に反映済み）：作成時にも地域を任意で聞く／都道府県は47の一覧から選ぶ（`features/household/prefectures.ts`、OS の pageSheet のシート）・市区町村は文字／切り替えの入口は世帯が1つでも出す／世帯の削除は OS の確認のアラート
- 選択中の世帯は `SelectedHouseholdProvider`（`(app)/_layout.tsx` で全体に配る）。`useSelectedHousehold()` の `state`（loading / error / none / selected）と `select()`。保存した世帯から外れていたら、最初の世帯にする（`selection.ts`）
- **ロールでの出し分けは `useCan('record' | 'edit' | 'manageHousehold')`**（`permissions.ts`、`screen-inventory.md` の 6.1）。16・17 の画面の編集・記録のボタンは、これで閲覧者に出さない。知らないロールは閲覧者として扱う
- どの世帯にも属していなければ、`(app)/_layout.tsx` が ONB-01（`/onboarding`）に移す。退出・削除で世帯がなくなったときも同じ
- 確認のアラートは `lib/confirm.ts`（`confirm()`・`notify()`）。ブラウザでは React Native Web の Alert が何もしないので、`confirm.web.ts` でブラウザの確認を使う（Claude in Chrome はダイアログで止まるので、ブラウザでは押さない）
- メンバーの操作（ロールの変更・移譲・外す）は、メンバーを押して開く画面（`household/members/[userId].tsx`）。アクションシートは Android でボタンの数に制限があるため、画面にした
- 招待から参加する（`/join`）は案内だけの仮の画面。中身は 15.2
- 新しい部品：`ListRow`・`FormField`・`RoleBadge`（`components/`）。`ListRow`・`FormField` は `foundations.md` の5章に追記した。チェックや「›」の印は文字で仮置き（アイコンの導入のあとで OS のシンボルにする）
- 世帯の一覧の取得に失敗したとき（`state.status === 'error'`）の画面は、まだない（ErrorState は 17 の今日の画面で作る）

## 15.2 で決めたこと（15.1 の確認での直しを含む）
- 15.2 はアプリ側だけ。本番の招待用ドメイン・Universal Links / App Links の検証ファイル・WEB-01 は 15.3 に分けた（2026-11-19 にユーザーと決めた）
- 招待リンク `https://<ドメイン>/i/<トークン>` は、`src/app/+native-intent.tsx` で `/invite/<トークン>` に読み替える。開発中は `xcrun simctl openurl <ID> uchinokusa://invite/<コード>` で開ける
- 未サインインで招待を開いたら、`features/invitation/pendingInvite.ts` に覚え、サインイン（初回は表示名の設定も）のあとに `homeHref()` で同じ参加の確認へ戻す。忘れるのは、参加・「今日を開く」・サインアウトのとき（参加の確認を開いた時点で忘れると、初回の表示名の設定をはさんだときに戻れないため）
- 招待コードの入力は、空白・ハイフンを除き、全角を半角・小文字を大文字にしてから確かめる（`inviteCode.ts`）。QR は、ドメインを問わず `/i/<トークン>` の形なら招待とみなす（`inviteLink.ts`。ドメインが決まる 15.3 で見直す）
- 有効期限は7日で固定（選ばせない）。招待の一覧に作成者は出さない（API になく、作れるのはオーナーだけ）
- 参加済みのとき、その世帯を開くには世帯の ID が要る → バックエンドに依頼中（下の依頼2）。それまでは「今日を開く」
- QR：表示は `react-native-qrcode-svg`、読み取りは `expo-camera`（カメラの許可の文言は `app.json` の plugins）。QR はダークでも明るい地にする（`color/qr/*` を `foundations.md` とトークンに足した）。シミュレーターにはカメラがないので、読み取りは実機で確かめる
- **タブは「今日・植物・設定」の3つ**（2026-11-19 にユーザーと決めた）。世帯の設定・メンバー・招待は `(tabs)/settings/` の中（URL は `/settings/household/...`）。タブの上に重ねる画面の戻るボタンは矢印だけ（前の画面の名前が「(tabs)」になるため）
- ヘッダーの世帯名は、iOS 26 のヘッダーのボタンのガラスの背景を消す（`householdSwitcherOptions` の `unstable_headerLeftItems` と `hidesSharedBackground`）
- 都道府県の選択は、RN の `Modal` ではなく、Expo Router の画面（`presentation: 'modal'`）。16.1 で、選択肢のシート全般（`ChoiceField` と `/choice`）にまとめた
- 新しい部品：`Toast`（保存しました、など。`ToastProvider` はルートのレイアウト）、`QRCodeView`、`QRScanner`。`ListRow` に右端の操作を置く `trailing` を足した。どれも `foundations.md` の5章に追記した
- 日付の表示は `lib/formatDate.ts`（「11月26日(木)」。相対の表現は【未決】）
- 未確認：表示名の入力で「日本語と英語の変換が逆」になったという報告（2026-11-19）。どの操作でどうなったかをユーザーに確かめている

## 16.1 で決めたこと
- **置き場所の管理は世帯の設定**（2026-11-20 にユーザーと決めた。はじめ植物のタブのヘッダーに置いたが、「植物一覧はあくまで植物のダッシュボードがメイン」「置き場所を追加するっていう機能はここに置くべきじゃない」とのことで移した）→ `(tabs)/settings/household/locations.tsx`（LOC-01）。閲覧者も開ける。植物の一覧では、置き場所は絞り込みにだけ使う
- 16.2 でやること：植物の登録・編集で置き場所を選ぶときに、その場で新しい置き場所を作れるようにする（ユーザーが希望。形は 16.2 で決める）
- 作成・編集（LOC-02）は `(app)/locations/new.tsx`・`[id].tsx` のモーダル。選択肢のシートを上に重ねるため、タブの中ではなく `(app)` のスタックに置いた
- 環境の選択肢は API の値のまま（8方位・距離3段階・遮光2種類）。屋外では窓からの距離と遮光の欄を隠し、保存で未設定に戻す。知らない値は未設定として扱う（`features/location/locationValidation.ts`）
- 選択肢のシートを共通の部品にした：`components/ChoiceField.tsx`（欄）と `(app)/choice.tsx`（シート）、`lib/choicePicker.ts`（開いた欄に値を返す）。都道府県もこれを使う（`/prefecture` と `prefecturePicker.ts` は消した）
- 削除の確認は、紐づく植物の数（`plantCount`）を出す。0株なら数を出さない。決定のボタンは「削除する」。保存・削除のあとはトーストを出してモーダルを閉じる
- TODO(16.2)：置き場所の変更・削除のあとに、植物の一覧のキャッシュも取り直す（`useLocations.ts`）

## 16.2 で決めたこと（4つに分けて進める。2026-11-20）
- 16.2a 一覧：次の予定日が近い順（予定なしは最後に名前順）。検索は OS のヘッダーの検索バー → API の `q`、置き場所の絞り込みはアプリの中（`features/plant/plantList.ts`）。作業の種類はバックエンドへの依頼3を待つ
- 16.2b 登録・編集：`(app)/plants/new.tsx`・`plants/[id]/edit.tsx`（モーダル）、種類の検索は `(app)/species.tsx`（`speciesPicker.ts` で値を返す）。置き場所の選択肢の最後の「＋ 新しい置き場所」は `newLocationFlow.ts` で LOC-02 を開き、作ったものを選ぶ
- 日付は `components/DateField.tsx`（`@expo/ui/community/datetime-picker`。iOS は compact、Android はダイアログ。ネイティブの追加はなし）。購入日は未来を選べない（サーバーはまだ受け付けるが、アプリで止める）
- 16.2c 植え替えの提案：`(app)/plants/[id]/repotting.tsx`（登録のモーダルを `router.replace` で置き換える）。提案の理由は `features/suggestion/suggestionText.ts` で材料から組み立てる。部品 `SuggestionCard` を足した（今日やること（17.1）でも使う）
- 16.2d 削除・アーカイブ：編集の画面の下に「アーカイブする」「植物を削除」（OS の確認のアラート → トースト → `router.dismissTo` で元の一覧へ）。アーカイブした植物（PLANT-05）は `(tabs)/plants/archived.tsx`（植物のタブの中）。入口は植物の一覧のいちばん下の行で、アーカイブした植物があり、検索・絞り込みをしていないときだけ出す（ユーザーが選んだ。たまにしか使わない入口をヘッダーに置かない。2026-11-21）。アーカイブの解除は詳細（PLANT-10）の「アーカイブ済み」の表示の中。アーカイブ・削除のあとは、世帯のキャッシュ（`['households', id]`）をまとめて取り直す（提案・今日のタスクも変わるため）
## 16.3 で決めたこと（2026-11-22）
- 植物の詳細（PLANT-10）は、OS のセグメントの切り替えで「概要・履歴・メモ・カレンダー」に分ける（ユーザーが選んだ）。概要に植物の情報とタスクの一覧。履歴・メモ・カレンダーは 17.3 まで「準備中」
- タスクの作成は `(app)/plants/[id]/tasks/new.tsx`（モーダル）。入力の決まりは `features/task/taskValidation.ts`（サーバーと同じ上限）、表示は `careTask.ts`・`taskText.ts`
- おすすめの初期値：task-defaults の実行条件（いまは水やりだけ）。周期の目安は植え替えの24か月だけ（0028）。ほかは出典がないので空
- 「季節ごとに変える」をオンにすると、基本の「周期」の欄を隠して4つの季節だけを入れる（ユーザーが選んだ案A。重ねて入れさせない）。API が必須の基本の周期には春の周期を送る。周期の数の欄のプレースホルダーは「例：7」
- タスクの編集（16.3b）は `(app)/plants/[id]/tasks/[taskId].tsx`（モーダル）。タスク1件の API がないので一覧から探す。予定日は変えたときだけ送る（送ると手で決めた日になり、季節の切り替えで計算し直さなくなる）。一時停止・再開・削除は下の行で、押したらトーストを出して閉じる
- 植え替えの注意（16.3c）：植え替えのタスクを作成・予定日を変えて保存したら、`useRepottingAvoidAlert` が提案を取り直し、`repotting.avoid` があれば OS のアラートを出す。詳細のタスクの項目には「注意：」の行（ListRow の `footer`、色は `suggestion/avoid`）。試しに、ポトスに予定日 2027-01-10 の植え替えのタスクを API で入れた
- 部品を足した：CareKindIcon（仮置きのシンボル）、Badge（RoleBadge はこれを使う）、IntervalInput、SeasonalIntervalEditor、TaskListItem。ListRow に `leading`、DateField に `minimumDate`
- 16.3c の植え替えの注意は、保存のあとに提案の一覧を取り直し、そのタスクの `repotting.avoid` があれば OS のアラートで理由を出し、詳細のタスクの項目にも注意の1行を残す（ユーザーが選んだ）
- 試しのデータ：ポトスに、水やり（季節ごとの周期・実行条件つき）、肥料、葉水（一時停止中）のタスクを API で入れた
- ヘッダーの文字のボタンは `components/HeaderTextButton.tsx` の `headerRightTextButton()`（背景なし）
- 試しのデータ：ローカルの API に、エミュレーター用の署名なしのトークン（`alg: none`、`iss`・`aud` は `demo-uchinokusa`、`user_id` は DB の `users.firebase_uid`）で呼べる。スクリプトはリポジトリに入れない

## 17 で決めたこと（2026-11-23）
- 17.1 今日やること（`(tabs)/today/index.tsx`）：API の `today` をそのまま並べ、まとめ方（予定日順・置き場所ごと・作業の種類ごと）は `features/today/todayList.ts`。表示の切り替えはヘッダーの右の OS のメニュー（iOS は `unstable_headerRightItems` の `menu`、Android は `@expo/ui` の MenuView）で、端末に保存する（ユーザーが選んだ）
- 部品を足した：TaskCard（完了のボタンは 17.2 で `trailing` に置く）、OverdueBadge、EmptyState
- 提案の文は `todaySuggestionText`（植え替えの注意は本文＋理由、知らない提案は種類ごとの一般的な文）
- 17.2 は 17.2a 完了と取り消し → 17.2b スキップと実施日を変えて記録 → 17.2c まとめて完了 に分ける（ユーザーが選んだ）
- 17.2a：完了は `features/today/useCompletion.ts` の `useCompleteTask`（今日の一覧から楽観的に外し、失敗で戻す）。画面の流れは今日の画面の `useCompletionFlow`（スナックバー・すでに完了のダイアログ・通信できないときのトースト）。取り消しは `useUndoRecord`。部品を足した：CompleteButton（丸いチェック。ユーザーが選んだ）、Snackbar（ルートの `SnackbarProvider`）。トークン `borderWidth.thick` を足した
- 17.2b：スキップ（`(app)/today/[taskId]/skip.tsx`）と実施日を変えて記録（`record.tsx`）は OS のシート。入口はカードの長押しのメニュー（ユーザーが選んだ。iOS は `Link.Menu`、Android は `Alert` の選択肢）。一度使うまで一覧の上に知らせを出す（端末に保存）。完了・スキップの流れは `features/today/useCompletionFlow.ts` に共通化。シートは開いたときのタスクを `useOpenedTodayItem` で覚える（楽観的更新で一覧から外れても中身が消えないように）
- 17.2c：まとめて完了は、ヘッダーの右端の「選択」から選択モードへ（ユーザーが選んだ）。選んだ状態は今日の画面（`TodayScreen`）の `selection`、結果の分け方は `features/today/batch.ts`、送るのは `useCompleteTasks`（楽観的に外す）。失敗した分は選んだまま残す。部品を足した：SelectionCheck
- 完了のボタンの mutation は、カードではなく一覧（`TodaySections`）に置く。カードは完了の直後に一覧から外れて消えるので、カードに置くと、そのあとのダイアログや取り消しが動かなくなるため
- 試しのデータ：寝室のサンスベリア（水やり、3日過ぎ）、ローズマリー（剪定、今日）、アイビー（肥料、今日、置き場所なし）のタスクを API で入れた

## 最終の QA で確かめること（ユーザーと決めた。2026-11-23）
- 17.2a：ほかのメンバーが先に完了していたときの確認（画面に出ているタスクを、API で先に完了させてから押す）
- 17.2a：通信できないときの知らせ（API サーバーを止めてから押す）

### CORS の中継（ブラウザで API を確かめるとき。バックエンドが CORS に対応するまで）
リポジトリには入れない。scratchpad などに次のような Node のスクリプトを置き、`http://localhost:8090` → `8080` に流して CORS のヘッダーを足す。web は `EXPO_PUBLIC_API_BASE_URL=http://localhost:8090/v1 npx expo start --web -c` で起動する
- OPTIONS には 204 と `Access-Control-Allow-Origin: http://localhost:8081`・`Allow-Headers: Authorization, Content-Type`・`Allow-Methods` を返し、ほかは `http.request` で 8080 に流して同じヘッダーを足す

## 14.3 で決めたこと
- トークンは `mobile/src/theme/tokens.ts`。画面では `useTheme()` から使う。文字は `AppText`（`variant`・`color`）、アプリの層のボタンは `Button`（`primary`・`secondary`）
- 名前の読み替えの追加の決まり（2026-11-09 にユーザーと決め、`.kiro/steering/design.md` の「名前をそろえる」に入れた）：
  - 子を持つトークンは、自身の値を `default` に置く：`color/accent` ⇔ `theme.color.accent.default`、`color/accent/subtle` ⇔ `theme.color.accent.subtle`
  - ハイフンはキャメルケース：`color/on-accent` ⇔ `theme.color.onAccent`、`text/large-title` ⇔ `theme.text.largeTitle`、`size/tap-min` ⇔ `theme.size.tapMin`
  - `color/care/<種類>/subtle` を足すときは、`care.watering` などを `{ default, subtle }` に変える
- `foundations.md` にないものとして、`borderWidth.hairline`（1pt。`shadow/card` の「1pt の枠」）と、ボタンを押している間・無効のときの不透明度 0.6 を使った
- foundations.md の「比」の列（コントラスト比）は、`src/theme/__tests__/tokens.test.ts` でトークンの値から確かめている
- ライトモードの見た目は、まだ確かめていない（この Mac がダークモードのため）。iOS シミュレーターで確かめる

## ユーザーと決めたこと（2026-11-08）
- **テストの進め方**：TDD にはしない。サクサク実装する
  - 毎回、型チェック（`tsc`）・lint・formatter が通ることを確かめる
  - ロジックとして切り出せていてテストできるもの（util、フォームのスキーマなど）は、Jest と Testing Library でユニットテストのファイルを作る
- **画面の確かめ方**：スマホの画面を、実際にブラウザで確かめながら進めることを大事にする
  - Expo の web（`npx expo start --web`）をスマホの幅で表示して確かめる（Claude は Chrome の操作ツールで見られる）
  - ブラウザでは `@react-native-firebase` が動かないので、開発では Firebase の JS SDK で Auth Emulator にサインインする（14.1。サーバーは Emulator のトークンを受け付ける）
  - iOS の標準部品は、ブラウザでは見た目が変わる。形と流れはブラウザで試し、最後の見た目は iOS シミュレーターで確かめる
- **18.1 で決めたこと（リアルタイム同期）**：`src/realtime/`。`RealtimeConnection`（接続・購読・30秒ごとの ping と10秒の pong 待ち・1秒から倍で最大30秒のつなぎ直し）は React から切り離してテストする。`RealtimeProvider` を `(app)/_layout.tsx` に置き、所属するすべての世帯を購読する。購読できたら世帯のデータを取り直し、世帯ごとのバージョン番号が飛んだら世帯をすべて取り直す。変更イベントは `invalidationTargets` で、世帯のキー（`['households', id]`）と植物の詳細（`['plants', plantId]`）を取り直す。ブラウザ（web）は接続のヘッダーを付けられないので、いまはつながない（0030 の「最初のメッセージでトークン」がバックエンドに要る。web を使うときに依頼する）
- **18.2 は手続き待ち（2026-11-24）**：プッシュ通知の受け取りは `@react-native-firebase/messaging`（tech.md）で、本番の Firebase のプロジェクトの `GoogleService-Info.plist` と、APNs の鍵の Firebase への登録が要る。ユーザーが Apple Developer Program と Firebase の手続きをしてから、18.2（通知設定の画面・許可・トークンの登録・通知を押したときの移動）をまとめて作る。ユーザーには手順（App ID `app.uchinokusa`、Push Notifications と Sign in with Apple、APNs の `.p8`・Key ID・Team ID）を伝えた。`.p8` はリポジトリに入れない
- **次の `/retro` で拾うこと（2026-11-24）**：メモの一覧を見て、ユーザーが「リストアイテムは角丸のカードにした方がいい」と言った。見た目の直しは 19.4 でまとめて行う。カレンダーの印（アイコン＋円の形で状態）を見て「状態は良く表現できている。デザインは直感的に意味が伝わるのが大事」と言った
- **デザインの振り返り（`/retro`）**：デザインの作業の区切り（1画面が終わったときなど）で `/retro` を呼ぶ。試行錯誤から好みを抜き出して `docs/design/preferences.md` を育て、デザインの作業の前に読む（`.claude/skills/retro/SKILL.md`）

## リリースの前に必ずやること
- 本番のビルド（iOS / Android）の認証を、ブラウザの開発用の Firebase の JS SDK から、スマホ専用の `@react-native-firebase/auth` に切り替える（`tasks.md` の 14.4）

## バックエンドからの申し送り
- アカウントの削除（`DELETE /v1/me`、19.2 の画面）：
  - Apple でサインインしたユーザーは、削除の直前に Apple で再認証し、受け取った認可コードを `appleAuthorizationCode` で送る（ないと 422 `required`、期限切れ・使用済みだと `invalid`）
  - ほかにメンバーのいる世帯の唯一のオーナーなら、`409 owner_transfer_required` と移すべき世帯の一覧が返るので、オーナー権限の移譲へ案内する
  - 削除したユーザーの記録・メモの作業者名は「退会したユーザー」になるが、その変更のイベントは出ない。**メンバーの削除（`member` の `deleted`）のイベントを受けたら、その世帯のデータを取り直す**
- 詳しくは `api/openapi.yaml` の `DELETE /me` と `docs/decisions/0035`

## 担当の分け方
- バックエンドとアプリは、別々のエージェントが同じ作業フォルダ・同じブランチ（`main`）で並行して進める
- **アプリ担当がコミットしてよいファイル**：`mobile/`（`openapi.yaml` から生成した `mobile/src/api/schema.d.ts` を含む。2026-11-08 にユーザーと決めた）、`HANDOFF-mobile.md`、`tasks.md` のアプリのタスク（14〜20）の行、`docs/design/screen-inventory.md`・`foundations.md`（`design.md` の決まりで、画面の実装の前に直すため。15.1 から）、デザインの振り返りのファイル（`.claude/skills/retro/`、`docs/design/preferences.md`、`docs/design/retro/`）、README と進捗（ルートの `README.md`、`scripts/update-progress.mjs`、`.githooks/`、`docs/roadmap.md`、`docs/progress/`。2026-11-23 にユーザーと決めた。`docs/roadmap.md` の「MVP：リリース準備（バックエンド）」のチェックはバックエンドも付ける）、アプリの開発ログ（`docs/devlog/`。面接で話すための不具合の調査などの記録。2026-11-24 にユーザーと決めた）
- コミットは、自分のファイルのパスを指定して行う。`git add -A` や `git add .` は使わない（もう片方の作りかけの変更が入るため）
  ```
  git add <自分のファイル>
  git commit -m "..." -- <自分のファイル>
  ```
- `docs/decisions/` に記録を足すときは、作る直前にフォルダの最新の番号を確かめる（もう片方も同じ番号で作ることがあるため）

## API の決まり
- **API の仕様は `api/openapi.yaml` が唯一の正（SSoT）**。アプリ担当は変えない
- 型は `make gen-api` で `mobile/src/api/schema.d.ts` に生成する。`openapi.yaml` を変えたバックエンド担当が一緒にコミットするのが基本だが、アプリ担当が生成してコミットしてもよい（生成の結果は同じになる）
- 実装していて API に問題があったら（足りない項目、使いにくい形など）、下の「バックエンドへの依頼」に書き、ユーザーに伝える。アプリ側で回避の作り込みはしない

## バックエンドへの依頼
バックエンド担当は、対応したら「対応済み」とコミットを書き添える。

### 1. CORS に対応してほしい（2026-11-08）
- ブラウザ（Expo の web、`http://localhost:8081`）から API（`http://localhost:8080`）を呼ぶと、事前確認の `OPTIONS /v1/me` が **405** になり、ブラウザがリクエストを止める（アプリには「通信できませんでした」と出る）
- ブラウザで画面を確かめながら進める方針（「ユーザーと決めたこと」）のため、REST API で CORS に対応してほしい
  - 許可するオリジンは、すでにある `ALLOWED_ORIGINS` を使う案（ローカルは `http://localhost:8081`）
  - 必要なヘッダー：`Authorization`、`Content-Type`。メソッド：`GET`・`POST`・`PUT`・`PATCH`・`DELETE`
  - 本番のアプリ（iOS / Android）はブラウザではないので CORS は関係しない。`ALLOWED_ORIGINS` が空なら、今までどおり何も許可しない、で足りるはず

### 2. 招待の内容（`InvitationPreview`）に、すでに参加している世帯の ID がほしい（2026-11-19）
- 参加の確認（INV-12）で `alreadyMember: true` のとき、画面の仕様は「すでにこの世帯に参加しています」と **その世帯を開くボタン** を出す（要件3-6）
- いまの `GET /invitations/{tokenOrCode}` は世帯名しか返さないので、アプリはどの世帯に切り替えればよいかわからない（同じ名前の世帯もありうる）
- 案：`InvitationPreview` に `householdId`（`alreadyMember` が true のときだけ。まだメンバーでない人には世帯の ID を見せない）を足す
- それまでは、アプリは「今日」を開くボタンにしておく

### 3. 植物（`Plant`）に、次の予定日の作業の種類がほしい（2026-11-20）
- 植物の一覧（PLANT-01）は、要件5-4で「次のお手入れの予定日」を出す。画面の仕様では **どの作業か**（水やり・肥料など）も出したい（例：「水やり 11月26日(木)」）
- いまの `Plant` には `nextDueOn`（日付）しかなく、どのタスクの日付かがわからない。植物ごとにタスクの一覧を取ると、200株で200回の呼び出しになる
- 案：`Plant` に `nextDueKind`（`CareKind`。`nextDueOn` と同じタスクのもの。同じ日に複数あれば、どれか1つでよい。`nextDueOn` がなければ項目がない）を足す
- それまでは、アプリは日付だけを出す（ユーザーと決めた。2026-11-20）

## デザイン
- 画面と部品の実装では、必ず `.kiro/steering/design.md` を読んで従う
- 画面の一覧は `docs/design/screen-inventory.md`、色や文字などの値は `docs/design/foundations.md`
- 未決：「植物」タブの名前、要件 9-5（置き場所ごと・種類ごとの表示の切り替え）の扱い。16.2 までに決める

## 進め方
- 1タスクずつ進め、各タスクの冒頭で「何を・なぜ」を短く説明し、終わったら止まってユーザーの確認をもらう
- タスクが終わったら `tasks.md` のチェックボックスを `[x]` にし、コミットする
- push は大きいタスク（14, 15 など）の単位で行う
