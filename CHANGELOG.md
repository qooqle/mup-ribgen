# CHANGELOG

## dev (unreleased)

### 実装された機能

#### フェーズ1: 基盤実装
- **IRデータモデル** (`pkg/ir`): Session Information、BGP RIB Info、Static Context データ構造定義
- **Static Context Manager** (`pkg/staticctx`): JSON 設定ファイルの読み込み、JSON Schema 検証
- **構造化ログ** (`pkg/logger`): log/slog ベースの JSON ログ、ログレベルフィルタリング
- **設定管理** (`pkg/config`): JSON 設定ファイル、環境変数オーバーライド、デフォルトパス

#### フェーズ2: DSL実装
- **DSLパーサー** (`pkg/dsl`): 再帰下降パーサー、行/列情報付きエラー
- **DSL Pretty Printer**: AST から整形 DSL テキストへの変換、ラウンドトリップ保証
- **DSL Linter**: 構文チェック、ベストプラクティス違反検出、修正提案
- **DSL Compiler**: AST から Dialect Transformer Go コード生成
- **DSL Test Harness** (`pkg/testharness`): 3 レベルテスト（単体/ライフサイクル/PCAP統合）

#### フェーズ3: Mode 1実装
- **PFCP Sniffer** (`pkg/pfcp`): gopacket/libpcap ベースのライブキャプチャ
- **PFCP PCAPFile Sniffer**: pcapgo ベースの PCAP ファイルリプレイ（libpcap 不要）
- **PFCP Parser**: Session Establishment/Modification/Deletion/Response メッセージ解析
- **PFCP Session State Manager**: Establishment/Modification/Deletion ライフサイクル管理、SEID エイリアス解決
- **Keysight N9 方言**: `dsl/keysight_n9.dsl` + コンパイル済みトランスフォーマー
- **Mode 1 Controller** (`pkg/mode1`): Sniffer → Parser → Transformer → SessionManager パイプライン

#### フェーズ4: GoBGP統合
- **IR Manager** (`pkg/ir`): SessionInfo + StaticCtx → BGPRIBInfo 合成、インメモリ RIB ストア
- **GoBGP gRPC Client** (`pkg/bgp`): 指数バックオフリトライ付き gRPC クライアント
- **MUP SAFI ルートビルダー**: Type 1 / Type 2 Session Transformed Route 生成
- **Pipeline Connector** (`pkg/pipeline`): Mode1Controller → IRManager → BGPSender 非同期接続

#### 動作確認ツール（フェーズ4補足）
- **メインバイナリ** (`cmd/mup-ribgen`): `--pcap`/`--interface` + `--dry-run` + `--config` サポート
- `--config` フラグで JSON 設定ファイルからモード選択・方言・GoBGP アドレスを読み込み

#### フェーズ5: Mode 2実装
- **Mode 2 gRPC Receiver** (`pkg/mode2`): free5GC SMF プラグインからのセッションイベント受信
  - JSON codec over gRPC（protoc 不要、JSON over `application/grpc+json`）
  - `ListenAndServe` でサーバ起動、context キャンセルで GracefulStop
  - `Receiver.ReportSession` が IRHandler（ir.Manager）に直接接続
- **Mode 2 gRPC Client** (`pkg/mode2`): テスト・プラグイン共用クライアント
- **free5GC SMF Integration Plugin** (`plugins/free5gc`): free5GC SMF 向けフックライブラリ
  - `MUPClient.OnSessionEstablished/Modified/Deleted` — セッション確立/更新/削除フック
  - 4G（GTPv2 S5-C）/ 5G（Nsmf REST）の制御プロトコル差異は free5GC 内部で吸収
- **Property 6-8**: Mode 2 セッション受信/更新/削除テスト（各 100 イテレーション）
- **gRPC 統合テスト**: Client → Receiver → mockIRHandler の完全パス検証

#### フェーズ6: 包括的テストとドキュメント
- **Property 1**: モード選択テスト (req 1.1, 1.4, 1.5)
- **Property 27**: CLI 引数テスト (req 11.2)
- **GoBGP リトライテスト**: withRetry の動作検証 (req 9.5)
- **エラーハンドリングテスト**: 不正 JSON、空ファイル (req 9.6)
- **ベンチマーク**: PFCP ~400ns/op、IR Manager ~300ns/op (req 10.2, 10.3 を大幅達成)
- **ドキュメント**: ユーザーマニュアル、開発者ガイド、プラグイン統合ガイド (README.md)

#### フェーズ7: リリース準備
- **DSL CLI 拡張** (`cmd/dslc`): `compile` / `lint` / `format` サブコマンド (req 5.3, 6.3, 6.5)
- **Makefile**: `build` / `test` / `bench` / `cover` / `cross` / `dist` / `generate` ターゲット
- **バージョニング**: `--version` フラグ、`-ldflags="-X main.version=..."` ビルド時注入
- **バグ修正**: `parseAddr` が `net.SplitHostPort` を使用するよう修正

### 実装済みプロパティ（28個中）

| プロパティ | 説明 | 要件 | 状態 |
|---|---|---|---|
| Property 1 | モード選択 | 1.1, 1.4, 1.5 | ✅ |
| Property 2 | PFCPパススルー | 2.6 | ✅ |
| Property 3 | PFCP Session Establishment | 2.3 | ✅ |
| Property 4 | PFCP Session Modification | 2.4, 2.8, 2.9 | ✅ |
| Property 5 | PFCP Session Deletion | 2.5 | ✅ |
| Property 6 | Mode2 Session Information受信 | 3.1, 3.3 | ✅ |
| Property 7 | Mode2 Session Information更新 | 3.4 | ✅ |
| Property 8 | Mode2 Session Information削除 | 3.5 | ✅ |
| Property 9 | Mode2 Session Passthrough | 2.6 | ⏭️ スキップ（Mode 1 専用 req） |
| Property 10 | Session Information データ完全性 | 4.2-4.5 | ✅ |
| Property 11 | DSL コンパイル | 5.3, 5.4, 5.5 | ✅ |
| Property 12 | DSL パース | 6.1 | ✅ |
| Property 13 | DSL パースエラー検出 | 6.2 | ✅ |
| Property 15 | DSL ラウンドトリップ | 6.4 | ✅ |
| Property 16 | DSL Linter 構文チェック | 6.5, 6.7 | ✅ |
| Property 17 | DSL Linter ベストプラクティス | 6.6, 6.7 | ✅ |
| Property 18 | Static Context 読み込み | 7.1-7.7 | ✅ |
| Property 19 | JSON Schema 検証 | 7.8, 7.9 | ✅ |
| Property 20 | BGP RIB 作成 | 8.2 | ✅ |
| Property 21 | BGP RIB 更新 | 8.3 | ✅ |
| Property 22 | BGP RIB 削除 | 8.4 | ✅ |
| Property 23 | BGP ルート属性完全性 | 8.5, 8.6, 8.7 | ✅ |
| Property 24 | エラーログ出力 | 9.1, 9.2 | ✅ |
| Property 25 | ログレベル設定 | 9.3 | ✅ |
| Property 26 | PFCP パースエラー継続 | 9.4 | ✅ |
| Property 27 | CLI 引数 | 11.2 | ✅ |
| Property 28 | DSL Test Harness 正確性 | 12.1-12.16 | ✅ |

### 既知の制限事項

- **free5GC 実環境テスト**: free5GC ローカル環境での動作確認は別途必要
- **libpcap 依存**: ライブキャプチャには libpcap が必要（PCAP リプレイは不要）
- **単一方言**: 現在 Keysight N9 方言のみ実装済み（他方言は DSL で追加可能）

### 将来の作業項目

- free5GC 実環境での動作確認（smf/context フック統合）
- 追加 PFCP 方言サポート
- Kubernetes/コンテナデプロイメント対応
- メトリクス出力（Prometheus）
- 設定ファイルのホットリロード（req 7.10）
