<!-- Inclusion Mode: Conditional: "mobile/**/*", "docs/design/**/*" -->
# デザインと画面の実装の方針

デザイン（Figma での作業を含む）と、`mobile/` の画面・部品の実装では、必ずこのファイルに従う。

## 参照するもの
| 何を | どこに |
|---|---|
| 必要な画面・部品・状態・挙動 | `docs/design/screen-inventory.md` |
| デザインから実装までの進め方（①〜⑥） | `docs/design/design-to-implementation.md` |
| デザインの判断の記録 | `docs/design/decisions/`（このファイルの方針の理由） |
| 見た目の正 | Figma（フレーム名は画面 ID。例：`TODAY-01`） |

## UI の2つの層（`docs/design/decisions/0001`）
- **OS の層**：OS の標準部品を使い、見た目を自作しない。iOS では HIG に従う
  - 画面遷移とナビゲーションバー、タブバー、確認のアラート、アクションシート・メニュー、日付の選択、共有シート、スイッチ、セグメントの切り替え、検索バー、シート（モーダル）、権限のダイアログ、触覚フィードバック
- **アプリの層**：uchinokusa のデザインシステム（トークンと部品）で作り、iOS と Android で同じ見た目にする
  - TaskCard、CareKindIcon、SuggestionCard、OverdueBadge、ConditionLabel、RoleBadge、一覧の項目、カレンダーの印、空の状態、取り消しのスナックバー、トースト、QRコードなど（`screen-inventory.md` の5章）
- 標準部品で表現できないことが出てきたら、OS の層の部品を自作する前に、ユーザーに相談する

## アプリの層の基準（HIG にそろえる）
- タップできる領域は 44pt 以上
- 文字の段階は HIG の文字スタイル（Large Title、Title、Headline、Body、Callout、Footnote など）にそろえる
- 端末の文字の大きさの設定（Dynamic Type）に追従し、文字を大きくしても崩れないようにする
- 作業の種類（5種類）は、色とアイコンの両方で区別する（色だけに頼らない）
- 赤（`color/danger`）は、取り消せない操作にだけ使う

## 名前をそろえる（Figma とコード）
- トークン：Figma の変数 `color/care/watering` ⇔ コードの `theme.color.care.watering`（`/` を `.` に読み替える）
- トークンの名前は役割でつけ、OS に依存させない（`color/accent` は可、`color/ios/systemBlue` は不可）
- 部品：Figma のコンポーネント名 ⇔ `mobile/src/components/` の部品名を同じにする
- 部品の状態：Figma のバリアントのプロパティ（`state=overdue`）⇔ props（`state="overdue"`）を同じにする
- 画面：Figma のフレーム名は `screen-inventory.md` の画面 ID。状態違いは `TODAY-01/empty` のように後ろに足す

## Figma への書き込み（デザインの下書き）
- 新しい図形を描かず、デザインシステムの部品と変数で組む
- OS の層は、Apple 公式の iOS の UI キットの部品を使う
- オートレイアウトを使い、レイヤーに意味のある名前をつける
- 端末の大きさは 393 × 852 を基本にする
- 通常の状態だけでなく、`screen-inventory.md` に書かれた状態（空・読み込み中・エラー・閲覧者など）のフレームも作る
- 書いたものは下書きとして扱い、ユーザーの確認を受ける

## Figma からの実装
- Figma MCP が返すコードの例は Web 向け（React + Tailwind）なので、そのまま使わず React Native に置き換える
- 色・余白・文字の大きさ・角丸は、テーマのトークンから使う。生の値（`#4A90D9`、`16`）を書かない
- `mobile/src/components/` にある部品を優先して使う。ない場合は、Figma の部品と同じ名前で作る
- OS の層にあたるものは、Figma で iOS の見た目に描かれていても、OS の標準部品で実装する（自作で再現しない）
- 文言は直書きせず、`ja.json` のキーから引く（Figma の文字列は例として扱う）
- 閲覧者・空・読み込み中・エラーの状態も、Figma のフレームと `screen-inventory.md` を見て作る
- 実装したら、シミュレーターの画面と Figma のスクリーンショットを、状態ごとに見比べる

## 変更の流し方
- 仕様の不足・矛盾に気づいたら、Figma やコードだけを直さず、仕様と `screen-inventory.md` を先に直す
- 新しい部品が必要になったら、画面の中だけで作らず、デザインシステムの部品として追加する
- 実機で気づいた見た目の問題は、Figma にも反映する
