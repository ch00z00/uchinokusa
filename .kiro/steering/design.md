<!-- Inclusion Mode: Conditional: "mobile/**/*", "docs/design/**/*" -->
# デザインと画面の実装の方針

デザイン（Figma での作業を含む）と、`mobile/` の画面・部品の実装では、必ずこのファイルに従う。

## 参照するもの
| 何を | どこに |
|---|---|
| 必要な画面・部品・状態・挙動 | `docs/design/screen-inventory.md` |
| トークン（色・文字・余白）、アイコン、部品の基準の値 | `docs/design/foundations.md`（Figma とコードの値の元） |
| デザインから実装までの進め方（①〜⑥） | `docs/design/design-to-implementation.md` |
| デザインの判断の記録 | `docs/design/decisions/`（このファイルの方針の理由） |
| 主な画面のおおまかな見た目 | `docs/design/screens/<画面 ID>.png`（Figma から書き出した画像） |
| オリジナルのアイコン | `docs/design/icons/*.svg` |
| ユーザーのデザインの好み（デザインの作業の前に読む） | `docs/design/preferences.md`（`/retro` で試行錯誤から育てる。回ごとの記録は `docs/design/retro/`） |

## UI の2つの層（`docs/design/decisions/0001`）
- **OS の層**：OS の標準部品を使い、見た目を自作しない。iOS では HIG に従う
  - 画面遷移とナビゲーションバー、タブバー、確認のアラート、アクションシート・メニュー、日付の選択、共有シート、スイッチ、セグメントの切り替え、検索バー、シート（モーダル）、権限のダイアログ、触覚フィードバック
- **アプリの層**：uchinokusa のデザインシステム（トークンと部品）で作り、iOS と Android で同じ見た目にする
  - TaskCard、CareKindIcon、SuggestionCard、OverdueBadge、ConditionLabel、RoleBadge、一覧の項目、カレンダーの印、空の状態、取り消しのスナックバー、トースト、QRコードなど（`screen-inventory.md` の5章）
- 標準部品で表現できないことが出てきたら、OS の層の部品を自作する前に、ユーザーに相談する

## アイコン（`docs/design/decisions/0002`）
- 一般的なアイコンは OS のシンボルを使う（iOS：SF Symbols、Android：Material Symbols Rounded）。対応表は `foundations.md` の4章
- アプリ固有の概念（作業の種類・提案の種類など）は、デザイナーが作ったオリジナルの SVG を両方の OS で使う。色はトークンで付ける
- SF Symbols を Android、ロゴ、アプリのアイコン、宣伝の素材に使わない（使用許諾の範囲外）

## アプリの層の基準（HIG にそろえる）
- タップできる領域は 44pt 以上
- 文字の段階は HIG の文字スタイル（Large Title、Title、Headline、Body、Callout、Footnote など）にそろえる
- 端末の文字の大きさの設定（Dynamic Type）に追従し、文字を大きくしても崩れないようにする
- 作業の種類（5種類）は、色とアイコンの両方で区別する（色だけに頼らない）
- 赤（`color/danger`）は、取り消せない操作にだけ使う

## 名前をそろえる（Figma・リポジトリ・コード）
- トークン：`foundations.md` の `color/care/watering` ⇔ Figma の変数 ⇔ コードの `theme.color.care.watering`（`/` を `.` に読み替える）
- トークンの名前は役割でつけ、OS に依存させない（`color/accent` は可、`color/ios/systemBlue` は不可）
- 部品：Figma のコンポーネント名 ⇔ `mobile/src/components/` の部品名を同じにする
- 部品の状態：Figma のバリアントのプロパティ（`state=overdue`）⇔ props（`state="overdue"`）を同じにする
- 画面：Figma のフレーム名と画像のファイル名は `screen-inventory.md` の画面 ID。状態違いは `TODAY-01/empty` のように後ろに足す

## Figma との関係（`docs/design/decisions/0003`）
- Figma は無料プランで使う。Figma MCP で Figma を読み書きしない
- Figma で作るのは、デザインシステムと、主な画面のおおまかな見た目だけ。すべての画面・状態の画像があるとは限らない
- トークンの値の正は `foundations.md`。Figma の値と食い違っていたら `foundations.md` を正とし、ユーザーに知らせる

## 画面の実装
- 実装の前に、`screen-inventory.md`（何を出すか）、`foundations.md`（どの値を使うか）、`docs/design/screens/` の画像（どう並べるか）を見る
- 画像がない画面は、デザインシステムと、似た画面の画像にならう
- 画像から読み取った見た目は、最も近いトークンに置き換える。画像から測った生の値を書かない
- 色・余白・文字の大きさ・角丸は、テーマのトークンから使う。生の値（`#4A90D9`、`16`）を書かない
- `mobile/src/components/` にある部品を優先して使う。ない場合は、デザインシステムの部品と同じ名前で作る
- OS の層にあたるものは、画像で iOS の見た目に描かれていても、OS の標準部品で実装する（自作で再現しない）
- 文言は直書きせず、`ja.json` のキーから引く（画像の文字列は例として扱う）
- 閲覧者・空・読み込み中・エラーの状態も、`screen-inventory.md` を見て作る（画像がなくても作る）

## 変更の流し方
- 仕様の不足・矛盾に気づいたら、コードだけを直さず、仕様と `screen-inventory.md` を先に直す
- 新しい部品が必要になったら、画面の中だけで作らず、デザインシステムの部品として追加し、`foundations.md` の5章に書き足す
- 実機での細部の調整は、コードだけを直してよい（細部の見た目の正はコード）
- トークンの値や部品の形が変わる調整は、`foundations.md` も直し、Figma のデザインシステムへの反映をユーザーに伝える
