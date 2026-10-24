# Docker Compose と Docker Swarm

- 日付：2026-10-24〜2026-10-25
- きっかけ：タスク 1.1 で `compose.yaml` を作ったときの疑問

## Q. `docker-compose.yml` ではなく `compose.yaml` という名前でもよくなったの？

よい。今の Docker Compose（v2、`docker compose` コマンド）では、`compose.yaml` が推奨の名前である。

`-f` を指定しないと、次の順番でファイルを探す。

1. `compose.yaml`（推奨）
2. `compose.yml`
3. `docker-compose.yaml`
4. `docker-compose.yml`（古い名前。互換のために今も読み込まれる）

`compose.yaml` と `docker-compose.yml` が両方あると、`compose.yaml` を使い、警告を出す。

あわせて変わったこと：

- ファイルの先頭の `version: "3.8"` は不要になった（書くと「obsolete」の警告が出る）
- コマンドは、ハイフンつきの `docker-compose`（v1、Python 製、サポート終了）から、スペース区切りの `docker compose`（v2、Docker CLI のプラグイン）に移った

## Q. なぜ Docker 専用ではない共通の仕様になったの？

Docker 以外でも、同じファイルを使いたいという声が増えたからである。

- Compose は、複数のコンテナをまとめて起動する手順を YAML に書く、Docker 社の道具だった。便利なので広く使われた
- そのうち Podman、Kubernetes（変換ツールの Kompose）、AWS ECS、Azure などでも「このファイルをそのまま動かしたい」という要望が出てきた
- しかし正式な仕様は Docker 社の中にしかなかった。ツールごとに、使える項目と使えない項目がばらばらになっていた
- そこで 2020 年に、Docker 社が Microsoft や AWS などと一緒に、ファイルの形式を **Compose Specification**（compose-spec.io）という公開の仕様にした

例えるなら、ある会社の社内フォーマットだった書類の書式を、業界の共通書式にしたようなもの。どの窓口に出しても、同じ書類が通るようになる。名前から `docker-` が外れたのも、このため。

## Q. `version` が obsolete（廃止）になったのはなぜ？

以前は、目的の違う2つの形式が並んでいた。

| 形式 | 向いていた用途 | 特徴 |
|---|---|---|
| 2.x | 1台のマシンでの開発 | `depends_on` の `condition` や、`mem_limit` などでのリソースの制限が書ける |
| 3.x | Docker Swarm（複数のマシン）での運用 | `deploy` が増えた。一方で、2.x の一部の機能が使えない |

**3.x は 2.x の上位互換ではなかった**。「新しいほうがよい」と思って 3 にすると、2 で使えた機能がなくなる。ここが分かりにくかった。

Compose Specification で 2.x と 3.x は1つの仕様にまとめられた。今の `docker compose` は常に最新の仕様でファイルを読み、使える項目は書かれている内容そのものから判断する。そのため、`version` を書いても動きが変わらなくなり、obsolete（書いても無視する）という扱いになった。

## Q. 2.x の `depends_on` は、どんな書き方で、どんな設定？

`depends_on` は、**サービスを起動する順番** を決める設定である。

### 短い書き方（リストで書く）

```yaml
services:
  api:
    build: .
    depends_on:
      - postgres
  postgres:
    image: postgres:16-alpine
```

`postgres` のコンテナを **起動してから** `api` を起動する。ただし、待つのはコンテナのプロセスが動き出すところまで。PostgreSQL が接続を受け付けられる状態になったかどうかは見ない。そのため、`api` が早すぎる接続に失敗することがある。

### 長い書き方（2.1 で追加された `condition`）

```yaml
services:
  api:
    build: .
    depends_on:
      postgres:
        condition: service_healthy   # postgres の healthcheck が成功するまで待つ
  postgres:
    image: postgres:16-alpine
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U uchinokusa -d uchinokusa"]
      interval: 5s
      timeout: 3s
      retries: 10
```

`condition` に書ける値：

| 値 | 意味 |
|---|---|
| `service_started` | 起動したら次へ進む（短い書き方と同じ） |
| `service_healthy` | `healthcheck` が成功する（healthy になる）まで待つ |
| `service_completed_successfully` | そのサービスが正常に終了するまで待つ（マイグレーションを先に済ませる用途など。のちに追加された） |

### 3.x と、今の仕様での扱い

- 3.x では `condition` が削除された。Swarm は `depends_on` を無視するので、「アプリ自身が接続を再試行すべき」という考え方だったため
- 今の Compose Specification では `condition` が復活していて、上の長い書き方がそのまま使える

### このプロジェクトでは

- `compose.yaml` の `postgres` には `healthcheck` を書いてある（`docker compose ps` で healthy かどうかが分かる）
- API サーバーは Compose ではなくホストで `go run` するので、今は `depends_on` を使っていない。将来サーバーも Compose に入れるときは、`condition: service_healthy` で PostgreSQL を待てる

## Q. 3.x の Docker Swarm とは？

**複数のマシンを1つのまとまり（クラスター）として扱い、そこにコンテナを配置して動かし続ける仕組み** である。Docker Engine に組み込まれている（Docker 1.12、2016 年からの「Swarm mode」）。

### Compose との違い

- **Compose**：1台のマシンで、複数のコンテナをまとめて起動する。「この台所で、この料理を作る」
- **Swarm**：複数のマシンにまたがって、決めた数のコンテナを動かし続ける。「チェーン店の全店舗で、この料理を常に3皿用意しておく。1店舗が閉まったら別の店舗で作り直す」

#### 図：Compose（1台のマシンの中だけ）

```
  $ docker compose up -d
          |
          v
  +-------------------- Mac --------------------+
  |                                             |
  |   +------------+      +----------------+    |
  |   |  postgres  |      | firebase-auth  |    |
  |   +------------+      +----------------+    |
  |                                             |
  +---------------------------------------------+
```

- コンテナが動くのは、コマンドを実行したマシンの上だけ
- そのマシンが止まると、すべて止まる

#### 図：Swarm（複数のマシンを1つのクラスターとして扱う）

```
  $ docker stack deploy -c compose.yaml myapp
          |
          v
  +-------------+
  |   manager   |   あるべき状態：api を3つ動かす
  +-------------+
     |    |    |    どのマシンで動かすかを決めて指示する
     v    v    v
  +----------+  +----------+  +----------+
  | worker 1 |  | worker 2 |  | worker 3 |
  |  [api]   |  |  [api]   |  |  [api]   |
  +----------+  +----------+  +----------+
```

#### 図：Swarm で1台が止まったとき（自己修復）

```
  +----------+  +----------+  +----------+
  | worker 1 |  | worker 2 |  | worker 3 |
  |  [api]   |  |   DOWN   |  |  [api]   |
  |          |  |    xx    |  |  [api]   | <- manager が作り直す
  +----------+  +----------+  +----------+

  動いている api は3つのまま
```

| | Compose | Swarm |
|---|---|---|
| 対象 | 1台のマシン | 複数のマシン（クラスター） |
| コマンド | `docker compose up` | `docker stack deploy` |
| 書くもの | 起動するコンテナ | あるべき状態（いくつ動かすか） |
| 止まったとき | 同じマシンで再起動するだけ（`restart` の設定） | 別のマシンで作り直す |
| 主な用途 | ローカル開発 | 本番の運用 |

### 主な用語

| 用語 | 意味 |
|---|---|
| ノード | クラスターに参加しているマシン |
| マネージャー | 全体の状態を管理し、どこで何を動かすかを決めるノード。複数台で合意を取りながら管理する（Raft という方式） |
| ワーカー | 指示されたコンテナを実際に動かすノード |
| サービス | 「このイメージを3つ動かす」という、あるべき状態の宣言 |
| タスク | サービスから作られる、1つひとつのコンテナ |
| スタック | 複数のサービスのまとまり。Compose のファイルから作る |

### 使い方の例

```sh
docker swarm init                             # このマシンをマネージャーにしてクラスターを作る
docker stack deploy -c compose.yaml myapp     # Compose のファイルからサービス群を配置する
docker service scale myapp_api=5              # api のコンテナを5つに増やす
```

`compose.yaml` の `deploy` に、Swarm 向けの設定を書く：

```yaml
services:
  api:
    image: example/api:1.0
    deploy:
      replicas: 3            # 常に3つ動かす
      update_config:
        parallelism: 1       # 1つずつ順番に新しい版へ入れ替える（ローリングアップデート）
      restart_policy:
        condition: on-failure
```

できること：

- コンテナやマシンが止まったら、別の場所で自動的に作り直す（自己修復）
- 1つずつ入れ替えて、止めずに新しい版にする（ローリングアップデート）
- どのノードに届いたリクエストも、動いているコンテナへ振り分ける（ルーティングメッシュ）
- パスワードなどを暗号化して配る（secrets）

### まだ残っている機能？

残っている。Swarm mode は今も Docker Engine に含まれていて、使える。

- ただし、複数のマシンでコンテナを動かす仕組みの主流は **Kubernetes** になった。Swarm を新しく選ぶ場面は少ない
- 2019 年に Docker 社の企業向け事業を買収した Mirantis が、Swarm の保守を続けると表明している
- Docker 1.12 より前の、別のツールとして提供されていた「Classic Swarm」は、開発が終わっている（Swarm mode とは別物）

設定が少なく覚えやすいので、小さな構成では今も使われる。

### Linux の機能と関係する？

**Swarm 自体は、Linux カーネルの機能ではない**。Docker Engine の中で動くプログラム（SwarmKit）である。ただし、その下で Linux の機能を使っている。

| Linux の機能 | 使われ方 |
|---|---|
| namespaces | コンテナごとに、プロセス・ネットワーク・ファイルの見え方を分ける（Docker のコンテナ全般の土台） |
| cgroups | コンテナが使える CPU やメモリを制限する（`deploy.resources` の制限の実体） |
| VXLAN | 別々のマシンのコンテナを、1つのネットワークにいるようにつなぐ（overlay ネットワーク） |
| IPVS | 届いたリクエストを、複数のコンテナに振り分ける（ルーティングメッシュの負荷分散） |
| iptables | 通信の転送や、ポートの公開 |

Mac の Docker Desktop も、内部で小さな Linux の仮想マシンを動かし、その上でコンテナを実行している。

## Q. 1台が止まって別の worker で作り直すと、その worker だけ負荷が高くならない？

なる。これは実際に起きる問題で、Swarm だけでは完全には防げない。いくつかの仕組みと、設計のしかたで対処する。

### 1. 作り直す場所は、なるべく空いているマシンが選ばれる

manager は、タスクが少ないノードを優先して配置する（spread という方式）。上の自己修復の図は、分かりやすくするために単純にしている。実際には、worker 1 と worker 3 のうち空いているほうに配置される。

ただし、3つの api を2台で動かすなら、どちらか1台は必ず2つ受け持つ。ここは仕組みでは避けられない。

### 2. リクエストは、マシンではなくコンテナごとに振り分けられる

```
               リクエスト
                   |
                   v
       +-----------------------+
       |     routing mesh      |   動いている api に順番に振り分ける
       +-----------------------+
          |                 |
          v                 v
    +----------+      +----------+
    | worker 1 |      | worker 3 |
    |  [api]   |      |  [api]   |
    |          |      |  [api]   |
    +----------+      +----------+
       1/3                2/3        <- 受け持つリクエストの割合
```

コンテナ1つあたりの負荷は変わらないが、2つ動いている worker 3 は、マシン全体として2倍の負荷になる。

### 3. 必要な資源を宣言して、詰め込みすぎを防ぐ

```yaml
deploy:
  replicas: 3
  resources:
    reservations:   # 確保が必要な量。空きがないマシンには配置しない
      cpus: "0.5"
      memory: 256M
    limits:         # 使ってよい上限。超えないように制限する（cgroups）
      cpus: "1"
      memory: 512M
  placement:
    max_replicas_per_node: 1   # 1台に1つまで
```

- `reservations` を書くと、空きのないマシンには配置しない。無理に詰め込んで全体が遅くなるより、配置を待つ（pending）ほうを選ぶ
- `max_replicas_per_node: 1` にすると、同じマシンに2つ置かない。その代わり、台数が足りないあいだは api が2つに減る

### 4. 1台減っても耐えられる台数にしておく（N+1）

いちばん基本の対策は、**1台止まっても残りのマシンで受け止められる余裕を、最初から持たせておく** こと。たとえば、3台がそれぞれ CPU の 60% を使っている状態で1台止まると、残り2台はそれぞれ 90% になる。各マシンの普段の使用率を 60% 以下に抑えておけば、1台減っても耐えられる。

### 注意：止まったマシンが戻っても、自動では配置し直さない

worker 2 が復旧しても、Swarm は動いているコンテナを移動しない（動いているものを止めると、そのリクエストが失敗するため）。worker 2 は空のまま、worker 3 は2つ受け持ったままになる。

```sh
docker service update --force myapp_api   # コンテナを1つずつ作り直し、配置をやり直す
```

新しい版をデプロイするときにも、配置がやり直される。

### まとめ

| 対策 | 仕組み |
|---|---|
| 空いているマシンを選ぶ | manager の配置の方式（spread） |
| リクエストを均等に分ける | ルーティングメッシュ（IPVS） |
| 詰め込みすぎを防ぐ | `reservations`、`limits`、`max_replicas_per_node` |
| 余裕を持たせる | 台数の設計（N+1） |
| 偏りを戻す | `docker service update --force` |

Kubernetes でも同じ問題があり、考え方はほぼ同じ（resources の requests と limits、Pod を分散させる設定など）。Cloud Run は、負荷に応じてインスタンスの数を自動で増減するので、マシンの台数を自分で考える必要がない。

## このプロジェクトとの関係

- このプロジェクトの `compose.yaml` は、ローカル開発用（PostgreSQL と Firebase Auth Emulator）である。本番は Cloud Run で動かすので、Swarm は使わない（`.kiro/steering/tech.md`）
- `version` は書かず、ファイル名は推奨の `compose.yaml` にしている
