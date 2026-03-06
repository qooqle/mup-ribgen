# Keysight特化処理と共通処理の境界

このドキュメントは、`pfcp-n9.pcap` を例に、どこまでが Keysight 方言依存で、どこからが共通パイプライン責務かを整理する。

## 結論

- Keysight特化:
  - PFCP JSON/PCAP の IE 解釈
  - PDR/FAR/QER の関連付け規則
  - `PFCPSessionState -> SessionInformation` 抽出（複数 Route Instance）
- 共通:
  - RouteKey 単位の IR 管理
  - Static Context 合成
  - Type1/Type2 の送信判定（必須項目チェック）
  - no-op UPDATE 抑止
  - GoBGP Add/Update/Delete 送信

## Keysight特化（DSLコンパイル成果物）

- DSL定義:
  - `dsl/keysight_n9.dsl`
- 生成/実装トランスフォーマー:
  - `pkg/dialect/keysight_n9_transformer.go`
- 責務:
  - Establishment/Modification/Deletion から `PFCPSessionState` 差分を構成
  - FAR単位の複数 `SessionInformation` を生成
  - `RouteKey = canonical_seid:far_id` を付与

## 共通（方言非依存）

- PFCP状態管理:
  - `pkg/pfcp/session_manager.go`
  - 差分マージ、SEID alias 正規化、削除処理
- IR管理:
  - `pkg/ir/manager.go`
  - RouteKey単位ストア、pending delete
- Pipeline/BGP送信:
  - `pkg/pipeline/pipeline.go`
  - 必須フィールド未充足時の Add/Update 抑止
  - no-op Update 抑止（RouteType+RouteKey の fingerprint 比較）
- GoBGPルート構築:
  - `pkg/bgp/routes.go`

## pfcp-n9での実データ適用（要点）

- `route_key=1:11`:
  - Frame 5 (Session Modification Request) で必要情報が揃い Add 実行
- `route_key=1:12`:
  - Frame 5 では FAR12 が `buff` で不完全
  - Frame 7 (Update FAR 12 with OHC) で必要情報が揃い Add 実行
- no-op update 抑止:
  - 差分がない Update は BGP Update を送らない

## dry-run 出力方針

`--dry-run` は「BGP送信可否の検証」に必要な情報を表示する。

- 共通:
  - `op`, `route_type`, `seid`, `route_key`, `far_id`, `network_instance`, `rd`, `rt`, `nexthop`
- Type1:
  - `ue_ip` or `ue_prefix`, `endpoint`, `teid`, `qfi`, `source_address` (存在時)
- Type2:
  - `endpoint`, `teid`, `endpoint_address_length`, `mup_extended_community` (存在時)

## 運用上の意味

- 方言追加時は DSL/Transformer を拡張すればよく、共通送信ポリシーは再実装不要。
- BGP churn 抑制（no-op UPDATE抑止）により、意図しない再広告を防止できる。
- 不完全な一時状態（PFCP差分適用途中）で誤広告しない。
