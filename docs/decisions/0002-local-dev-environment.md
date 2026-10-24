# 0002 ローカル開発環境を Docker Compose と Firebase のデモプロジェクトで作る

- 状態：採用
- 日付：2026-10-24

## 背景
サーバーは PostgreSQL と Firebase Auth（ID トークンの検証）に依存する。開発やテストのたびに本物の Firebase プロジェクトやクラウドの DB に接続するのは、準備の手間がかかり、秘密情報の扱いも増える。

## 検討した案
1. **Docker Compose で PostgreSQL と Firebase Auth Emulator を起動する**（Emulator は Node のイメージに `firebase-tools` を入れて自前でビルドする）
2. PostgreSQL は Docker、Emulator はホストに `firebase-tools` を入れて `firebase emulators:start` で起動する
3. 開発用の本物の Firebase プロジェクトを作って使う

## 決定
案1を採用する。Firebase のプロジェクト ID には `demo-uchinokusa` を使う。

## 理由
- `docker compose up -d` の1つのコマンドで、必要な外部サービスがすべてそろう
- `demo-` で始まるプロジェクト ID は、Firebase の「デモプロジェクト」として扱われ、実在のプロジェクトや認証情報なしで Emulator だけで動く。誤って本番に接続する心配がない
- Auth の Emulator は Java を必要としないため、Node のイメージだけで済む
- `firebase-tools` のバージョンを Dockerfile で固定でき、開発者ごとの差が出ない

## トレードオフ
- Firebase に公式の Emulator 用イメージがないため、Dockerfile を自分で保守する（`firebase-tools` の更新は手動）
- 初回はイメージのビルドに時間がかかる
