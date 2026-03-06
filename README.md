# mup-ribgen

SRv6 MUP Controller — モバイルネットワークのセッション情報を収集し、GoBGP 経由で MUP SAFI RIB として配信するコントローラー。

## 概要

- **Mode 1 (Passive)**: SMF-UPF 間の PFCP トラフィックをパッシブスニッフィング
- **Mode 2 (Active)**: free5GC SMF からgRPCプラグイン経由でセッション情報を受信（フェーズ5実装予定）
- PFCP 方言の差異を DSL で吸収し、GoBGP に MUP SAFI ルートとして配信

---

## ユーザーマニュアル

### インストールとセットアップ

#### 前提条件

- Go 1.24 以上（[mise](https://mise.jdx.dev/) で自動管理）
- macOS または Linux
- libpcap（ライブキャプチャ使用時のみ）

#### 手順

```sh
# mise のインストール（macOS）
brew install mise

# shell への有効化
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc && source ~/.zshrc

# リポジトリをクローン
git clone https://github.com/qooqle/mup-ribgen.git
cd mup-ribgen

# Go バージョンをインストール
mise trust && mise install

# ライブキャプチャ用ライブラリ（オプション）
brew install libpcap

# ビルド
go build -o mup-ribgen ./cmd/mup-ribgen
```

---

### 設定ファイル形式

#### Static Context 設定（`static_context.json`）

Static Context は、Network Instance ごとの BGP ルート属性を定義します。

```json
[
  {
    "network_instance": "n9-nw",
    "rd": "65000:1",
    "rt": ["65000:100"],
    "nexthop_address": "192.168.1.1",
    "source_address": "2001:db8::1"
  },
  {
    "network_instance": "internet",
    "rd": "65000:2",
    "rt": ["65000:200"],
    "nexthop_address": "192.168.1.1",
    "endpoint_address_length": 64,
    "mup_extended_community": {
      "segment_identifier": "000100000002"
    }
  }
]
```

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `network_instance` | string | ✅ | PFCP の Network Instance 名 |
| `rd` | string | ✅ | Route Distinguisher（例: `"65000:1"`） |
| `rt` | []string | ✅ | Route Target リスト |
| `nexthop_address` | string | ✅ | BGP Next-hop アドレス |
| `source_address` | string | ❌ | Type 1 ルートの SRv6 Source Address |
| `endpoint_address_length` | int | ❌ | Type 2 ルートの Endpoint Address Length（ビット数） |
| `mup_extended_community` | object | ❌ | MUP Extended Community |

#### メイン設定ファイル（`config.json`）

```json
{
  "mode1_enabled": true,
  "mode2_enabled": false,
  "dialect": "Keysight_N9",
  "log_level": "INFO",
  "static_context_file": "static_context.json",
  "gobgp_address": "127.0.0.1:50051",
  "pfcp_interface": "eth0"
}
```

環境変数でのオーバーライドも可能：

| 環境変数 | 対応フィールド |
|---|---|
| `MUP_CONFIG_PATH` | 設定ファイルのパス |
| `MUP_LOG_LEVEL` | `log_level` |
| `MUP_GOBGP_ADDR` | `gobgp_address` |

---

### コマンドラインオプション

```
mup-ribgen [OPTIONS]

OPTIONS:
  --pcap FILE              PCAP ファイルをリプレイ（Mode 1、ライブキャプチャ不要）
  --interface IFACE        ライブキャプチャのネットワークインターフェース
  --static-context FILE    Static Context JSON ファイルのパス（デフォルト: static_context.json）
  --dialect NAME           PFCP 方言トランスフォーマー名（デフォルト: Keysight_N9）
  --route-type TYPE        MUP SAFI ルートタイプ: type1 または type2（デフォルト: type1）
  --gobgp-addr HOST:PORT   GoBGP デーモンの gRPC アドレス（デフォルト: 127.0.0.1:50051）
  --dry-run                BGP ルートを GoBGP に送信せず stdout に出力
  --log-level LEVEL        ログレベル: debug, info, warn, error（デフォルト: info）
  --channel-buffer N       イベントチャネルバッファサイズ（デフォルト: 128）
  --version                バージョン情報を表示して終了
  --help                   ヘルプメッセージを表示して終了
```

---

### Mode 1 使用方法

#### PCAP ファイルを使用したドライラン（GoBGP 不要）

```sh
mup-ribgen \
  --pcap sample/Keysight/pfcp-n9.pcap \
  --static-context static_context.json \
  --dialect Keysight_N9 \
  --dry-run
```

出力例：

```json
{"op":"UPDATE","route_type":"type1","seid":1,"route_key":"1:11","far_id":11,"network_instance":"n9-nw","ue_ip":"172.16.0.1","endpoint":"20.0.90.11","teid":1,"qfi":5,"rd":"65000:1","nexthop":"192.168.1.1"}
```

#### ライブキャプチャ + GoBGP 送信

```sh
sudo mup-ribgen \
  --interface eth0 \
  --static-context static_context.json \
  --dialect Keysight_N9 \
  --gobgp-addr 127.0.0.1:50051 \
  --route-type type1
```

> **注意**: ライブキャプチャには root 権限または `CAP_NET_RAW` ケーパビリティが必要です。

#### グレースフルシャットダウン

`SIGTERM` または `SIGINT`（Ctrl+C）でシャットダウンします：

```sh
kill -SIGTERM <pid>
# または
Ctrl+C
```

---

### トラブルシューティング

| 症状 | 原因 | 対処 |
|---|---|---|
| `dialect not found: Keysight_N9` | 方言名の大文字小文字が違う | `--dialect Keysight_N9`（正確な名前を指定） |
| `failed to load static context` | JSON ファイルのパスまたは形式が誤り | ファイルの存在とスキーマを確認 |
| `cannot connect to GoBGP` | GoBGP デーモンが起動していない | `gobgpd` を起動するか `--dry-run` を使用 |
| `event channel full, dropping event` | 処理が追いつかない | `--channel-buffer` を増やす |
| BGP ルートが生成されない | NetworkInstance が不明 | Static Context に該当する `network_instance` が設定されているか確認 |

---

## デプロイメントガイド

### システム要件

| 項目 | 要件 |
|---|---|
| OS | Linux（推奨）、macOS |
| Go | 1.24 以上 |
| libpcap | ライブキャプチャ使用時のみ（`apt install libpcap-dev` / `brew install libpcap`）|
| メモリ | 10,000 セッション管理時: 2 GB 以下 |
| CPU | 通常動作時 50% 以下（4 コア CPU 想定）|
| ネットワーク | GoBGP デーモンへの gRPC 通信（デフォルト: 50051/tcp）|

### インストール

```sh
# バイナリビルド（バージョン情報を埋め込む）
make build VERSION=v1.0.0

# または GitHub Releases からダウンロード（将来対応予定）
tar -xzf mup-ribgen-v1.0.0-linux-amd64.tar.gz
sudo install -m 755 mup-ribgen /usr/local/bin/
sudo install -m 755 dslc /usr/local/bin/
```

### GoBGP 統合セットアップ

1. GoBGP をインストール・起動します:

```sh
# GoBGP インストール
go install github.com/osrg/gobgp/v3/cmd/gobgpd@latest

# gobgpd.conf（最小設定）
cat > gobgpd.conf << 'EOF'
[global.config]
  as = 65000
  router-id = "192.168.1.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.1.2"
    peer-as = 65001

[[defined-sets.prefix-sets]]
  prefix-set-name = "mup-prefixes"

[[mrt-dump]]
EOF

gobgpd -f gobgpd.conf
```

2. mup-ribgen の設定ファイルを作成します:

```json
{
  "mode1_enabled": true,
  "dialect": "Keysight_N9",
  "log_level": "INFO",
  "static_context_file": "/etc/mup-ribgen/static_context.json",
  "gobgp_address": "127.0.0.1:50051",
  "pfcp_interface": "eth0"
}
```

3. 起動します:

```sh
sudo mup-ribgen --config /etc/mup-ribgen/config.json
```

### ネットワーク設定

- **Mode 1**: mup-ribgen はネットワーク的に SMF と UPF の間に配置するか、またはミラーポートに接続します。PFCP トラフィックをパッシブに受信するだけで、転送には影響しません。
- **必要なポート**: GoBGP gRPC 50051/tcp（ローカル接続）、PFCP 8805/udp（スニッフィング対象）

### セキュリティ考慮事項

- **最小権限**: ライブキャプチャには `CAP_NET_RAW` ケーパビリティが必要。root での常駐運用は避け、systemd の `AmbientCapabilities=CAP_NET_RAW` を使用してください。
- **設定ファイルの保護**: `static_context.json` には RD/RT などの BGP 設定が含まれるため、権限を `0600` に設定してください。
- **ネットワーク分離**: GoBGP との通信はローカルループバック推奨。リモート接続する場合は TLS を検討してください。

### systemd サービス例

```ini
[Unit]
Description=MUP RIB Generator
After=network.target gobgpd.service

[Service]
Type=simple
User=mupribgen
AmbientCapabilities=CAP_NET_RAW
ExecStart=/usr/local/bin/mup-ribgen --config /etc/mup-ribgen/config.json
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

---

## 開発者ガイド

### アーキテクチャ

```
PCAP / Interface
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  Mode 1 Controller (pkg/mode1)                              │
│  PFCP Sniffer → PFCP Parser → Dialect Transformer          │
│                              → PFCP Session State Manager   │
│                              → SessionEvent channel         │
└───────────────────────┬─────────────────────────────────────┘
                        │ SessionEvent
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  IR Manager (pkg/ir)                                        │
│  SessionInformation + StaticContext → BGPRIBInfo            │
│  In-memory store (map[SEID]*BGPRIBInfo)                     │
│  BGPEvent channel                                           │
└───────────────────────┬─────────────────────────────────────┘
                        │ BGPEvent
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  BGP Client (pkg/bgp)                                       │
│  GoBGP gRPC: AddPath / DeletePath                           │
│  withRetry: 指数バックオフで最大 MaxRetries 回リトライ       │
└─────────────────────────────────────────────────────────────┘
```

### パッケージ構造

| パッケージ | 説明 |
|---|---|
| `pkg/ir` | Session Information / BGP RIB Info データモデル、IR Manager |
| `pkg/pfcp` | PFCP パーサー、Session State Manager、Sniffer インターフェース |
| `pkg/mode1` | Mode 1 コントローラー（Sniffer → Parser → SessionManager の接続） |
| `pkg/bgp` | GoBGP gRPC クライアント、MUP SAFI ルートビルダー |
| `pkg/pipeline` | コンポーネント間の goroutine ブリッジ |
| `pkg/staticctx` | Static Context Manager（JSON 設定ファイルの読み込み） |
| `pkg/dialect` | Dialect Transformer インターフェースとレジストリ |
| `pkg/dsl` | DSL パーサー、コンパイラー、Linter、Pretty Printer |
| `pkg/dslruntime` | DSL コンパイル済みトランスフォーマーのランタイムサポート |
| `pkg/testharness` | DSL 変換ロジックのテストフレームワーク（3 レベル） |
| `pkg/config` | アプリケーション設定ファイル管理 |
| `pkg/logger` | 構造化ログ（log/slog ラッパー） |
| `cmd/mup-ribgen` | メインバイナリ |
| `cmd/dslc` | DSL コンパイラー CLI |

### 主要な設計判断

1. **非同期パイプライン**: コンポーネント間はバッファ付きチャネルで接続。バックプレッシャーは drop + WARN で対処（ロスレスより可用性を優先）。
2. **SEID エイリアス機構**: パッシブスニッフィングでは CP SEID（Establishment Request）と UP SEID（Establishment Response の F-SEID IE）が異なる。Modification/Deletion は UP SEID を使用するため、`RegisterSEIDAlias` で解決。
3. **DSL ビルド時コンパイル**: 方言変換ロジックは実行時ではなくビルド時にコンパイル。`go generate` で DSL → Go コードを生成。

### テスト戦略

- **プロパティベーステスト** ([gopter](https://github.com/leanovate/gopter)): 28 個の正確性プロパティを各 100 回以上検証
- **単体テスト**: 特定のエッジケース、エラー条件
- **DSL Test Harness**: 3 レベル（単体 / ライフサイクル / PCAP 統合）
- **ベンチマーク**: `go test -bench=.` で性能特性を計測

```sh
# 全テスト実行
go test ./...

# ベンチマーク実行
go test -bench=. ./pkg/pfcp/ ./pkg/ir/

# 特定のプロパティテストのみ
go test -run TestProperty ./...
```

### 新しい PFCP 方言の追加方法

1. **DSL ファイルを作成** (`dsl/<vendor>_<interface>.dsl`)：

```dsl
dialect "VendorX_N9" version "1.0"

establishment_to_state {
  pdr[pdr_id].ue_ip_address = fields.pfcp.create_pdr.ue_ip_address.address
  # ... その他のマッピング
}

state_to_session_info {
  ue_ip_address = state.pdr[1].ue_ip_address
  # ... その他のマッピング
}
```

2. **コンパイル**:

```sh
./dslc dsl/vendorx_n9.dsl -o pkg/dialect/vendorx_n9_transformer.go
```

3. **テストデータを作成** (`test_data/<dialect>/`):

```
test_data/vendorx_n9/
├── unit/          # 単体テスト JSON
├── lifecycle/     # ライフサイクルテスト JSON
└── expected/      # PCAP 期待値 JSON
```

4. **Test Harness で検証**:

```sh
go test ./pkg/testharness/... -dialect vendorx_n9
```

---

## プラグイン統合ガイド（Mode 2）

> **注**: Mode 2 は将来実装予定。現時点では Mode 1 のみ利用可能。

Mode 2 では、free5GC SMF からのセッション情報を gRPC push 経由で受信します。free5GC SMF 側にクライアントプラグインを統合し、mup-ribgen の gRPC サーバへ Session Information を push します。4G（GTPv2 S5-C）/5G（Nsmf REST）の制御プロトコル差異は free5GC SMF 内部で吸収されます。

```go
// Mode 2 プラグインが実装すべきインターフェース（将来）
type SessionInformationSender interface {
    HandleCreate(info *ir.SessionInformation) error
    HandleUpdate(info *ir.SessionInformation) error
    HandleDelete(seid uint64)
}
```

---

## 開発環境のセットアップ

```sh
# mise のインストール（macOS）
brew install mise

# shell への有効化
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc && source ~/.zshrc

# リポジトリをクローン後
mise trust && mise install

# システムライブラリ（Mode-1 ライブキャプチャ用）
brew install libpcap

# 依存パッケージ
go mod tidy

# ビルド確認
go build ./...
```

### バージョン管理

| ツール | バージョン管理ファイル |
|---|---|
| Go | `.mise.toml` |
| Go ライブラリ | `go.mod` / `go.sum` |

## プロジェクト構造

```
mup-ribgen/
├── cmd/
│   ├── mup-ribgen/        # メインバイナリ（Mode 1）
│   └── dslc/              # DSL コンパイラー CLI
├── pkg/
│   ├── ir/                # Session Information / IR Manager
│   ├── pfcp/              # PFCP パーサー・State Manager
│   ├── mode1/             # Mode 1 コントローラー
│   ├── bgp/               # GoBGP gRPC クライアント・ルートビルダー
│   ├── pipeline/          # コンポーネント接続 goroutine
│   ├── staticctx/         # Static Context Manager
│   ├── dialect/           # Dialect Transformer レジストリ
│   ├── dsl/               # DSL パーサー・コンパイラー
│   ├── dslruntime/        # DSL ランタイムサポート
│   ├── testharness/       # DSL テストハーネス
│   ├── config/            # 設定ファイル管理
│   └── logger/            # 構造化ログ
├── dsl/                   # DSL 定義ファイル（*.dsl）
├── test_data/             # DSL Test Harness 用テストデータ
├── sample/                # サンプル PCAP データ
├── static_context.example.json
├── .mise.toml
├── go.mod
└── go.sum
```
