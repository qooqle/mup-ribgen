# mup-ribgen

SRv6 MUP Controller — モバイルネットワークのセッション情報を収集し、GoBGP 経由で MUP SAFI RIB として配信するコントローラー。

## 概要

- **Mode 1 (Passive)**: SMF-UPF 間の PFCP トラフィックをパッシブスニッフィング
- **Mode 2 (Active)**: 既存 SMF 実装（free5GC, Open5GS 等）からプラグイン経由でセッション情報を受信
- PFCP 方言の差異を DSL で吸収し、GoBGP に MUP SAFI ルートとして配信

## 開発環境のセットアップ

### 前提条件

- macOS（Apple Silicon / Intel どちらも対応）
- [Homebrew](https://brew.sh/) がインストール済みであること

### 手順

#### 1. mise をインストール

```sh
brew install mise
```

#### 2. mise を shell に有効化

使用している shell に応じて `~/.zshrc` または `~/.bashrc` に以下を追加：

```sh
# zsh の場合
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc
source ~/.zshrc

# bash の場合
echo 'eval "$(mise activate bash)"' >> ~/.bashrc
source ~/.bashrc
```

#### 3. リポジトリをクローン

```sh
git clone https://github.com/qooqle/mup-ribgen.git
cd mup-ribgen
```

#### 4. Go をインストール（mise が .mise.toml を参照して自動でバージョンを決定）

```sh
mise trust   # 初回のみ必要
mise install
```

#### 5. システムライブラリをインストール（Mode-1 パケットキャプチャ用）

```sh
brew install libpcap
```

#### 6. Go 依存パッケージをインストール

```sh
go mod tidy
```

#### 7. ビルド確認

```sh
go build ./...
```

### バージョン管理

| ツール | バージョン管理ファイル |
|---|---|
| Go | `.mise.toml` |
| Go ライブラリ | `go.mod` / `go.sum` |
| システムライブラリ | `Brewfile`（将来追加予定） |

## プロジェクト構造

```
mup-ribgen/
├── cmd/
│   └── mup-controller/    # メインアプリケーションエントリーポイント
├── pkg/
│   ├── ir/                # Session Information（IR）データモデル
│   ├── pfcp/              # PFCP データ構造（Mode-1 用）
│   ├── dialect/           # Dialect Transformer インターフェース
│   ├── staticctx/         # Static Context Manager
│   ├── bgp/               # GoBGP gRPC クライアント
│   ├── config/            # 設定ファイル管理
│   └── logger/            # 構造化ログ
├── dsl/                   # DSL 定義ファイル（*.dsl）
├── test/                  # 統合テスト
├── test_data/             # DSL Test Harness 用テストデータ
├── docs/                  # ドキュメント
├── .mise.toml             # Go バージョン固定（mise）
├── go.mod
└── go.sum
```

## 開発

```sh
# テスト実行
go test ./...

# ビルド
go build -o bin/mup-controller ./cmd/mup-controller
```
