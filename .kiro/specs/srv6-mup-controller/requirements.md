# 要件定義書

## はじめに

本ドキュメントは、SRv6 MUP（Mobile User Plane）のMUP Controllerの要件を定義する。MUP Controllerは、モバイルネットワークのセッション情報（Session Information / IR）を収集し、静的コンテクストと合成してGoBGPのMUP SAFI RIBに変換して出力するシステムである。

## 用語集

- **SRv6**: Segment Routing over IPv6。IPv6ベースのセグメントルーティング技術
- **MUP**: Mobile User Plane。モバイルネットワークのユーザープレーンをSRv6で実現する技術
- **PFCP**: Packet Forwarding Control Protocol。SMFとUPF間の制御プロトコル（3GPP TS 29.244）
- **PFCP方言**: PFCPの実装による差異。ベンダー固有の拡張や解釈の違いを指す
- **SMF**: Session Management Function。5Gコアネットワークのセッション管理機能
- **UPF**: User Plane Function。5Gコアネットワークのユーザープレーン機能
- **MME**: Mobility Management Entity。4G LTEのモビリティ管理エンティティ
- **AMF**: Access and Mobility Management Function。5Gコアネットワークのアクセス・モビリティ管理機能
- **GoBGP**: Go言語で実装されたBGPデーモン
- **SAFI**: Subsequent Address Family Identifier。BGPのアドレスファミリー識別子
- **RIB**: Routing Information Base。ルーティング情報ベース
- **MUP_SAFI**: GoBGPにおけるMUP用のSAFI実装
- **Type_1_Session_Transformed_Route**: MUP SAFIのルートタイプ1。セッション変換ルート
- **Type_2_Session_Transformed_Route**: MUP SAFIのルートタイプ2。セッション変換ルート
- **Session_Information**: セッション情報の統一表現。UE IP、TEID、QFI、Endpoint、Network Instance等を含む。IRとも呼ばれる
- **IR**: Intermediate Representation（中間表現）。Session_Informationの別名
- **BGP_RIB_Info**: Session_InformationとStatic_Contextを合成した完全なBGPルート情報
- **DSL**: Domain Specific Language。PFCP方言とSession_Information間の変換を記述する専用言語（Mode_1専用）
- **PFCP_Session_State**: PFCPセッションの現在状態。PDR、FAR、QER等のIE情報を保持
- **Dialect_Transformer**: PFCP方言固有の変換ロジック。DSLからコンパイルされて生成される
- **PFCP_Session_State_Manager**: PFCPセッションのライフサイクルとステート管理を行うコンポーネント
- **DSL_Parser**: DSL定義ファイルを解析してASTを生成するパーサー
- **DSL_Pretty_Printer**: ASTを整形されたDSL定義ファイルに変換するプリティプリンター
- **DSL_Test_Harness**: DSL変換ロジックのテストフレームワーク。3つのレベル（単体、ライフサイクル、PCAP統合）でテストを実行
- **Network_Instance**: ネットワークインスタンス。論理的なネットワーク分離単位
- **RD**: Route Distinguisher。VPNルートを一意に識別する識別子
- **RT**: Route Target。VPNルートのインポート/エクスポートを制御する属性
- **MUP_Extended_Community**: BGP MUP固有のExtended Community。Direct-Type Segment Identifierを含む
- **Segment_Identifier**: Direct-Type Segment用の6バイトの識別子
- **Source_Address**: Type 1 ST RouteのSRv6ヘッダーで使用するソースアドレス
- **Endpoint_Address_Length**: Type 2 ST Routeの集約レベルを制御する長さ（IPv4: 32-64ビット、IPv6: 128-160ビット）
- **GTPv2**: GPRS Tunneling Protocol version 2。4G/5Gの制御プレーンプロトコル
- **Nsmf_API**: SMFが提供するサービスベースインターフェース（5G）
- **ISD**: Interworking Segment Discovery。本プロジェクトのスコープ外
- **DSD**: Direct Segment Discovery。本プロジェクトのスコープ外
- **JSON_Schema**: JSONデータの構造を定義するスキーマ言語
- **MUP_Controller**: 本システム。モバイルセッション情報を収集しGoBGP RIBに変換するコントローラー
- **Mode_1**: パッシブモード。SMF-UPF間のPFCPトラフィックを盗聴する動作モード
- **Mode_2**: アクティブモード。MME/AMFからの要求を受信しSMFとして振る舞う動作モード
- **Session_Establishment**: セッション確立処理
- **Session_Deletion**: セッション削除処理
- **Session_Modification**: セッション更新処理

## 要件

### 要件1: 動作モードの選択

**ユーザーストーリー:** システム管理者として、ネットワーク構成に応じて適切な動作モードを選択したい。これにより、既存のネットワーク機器への影響を最小限に抑えながらMUP機能を導入できる。

#### 受入基準

1. THE MUP_Controller SHALL Mode_1とMode_2を独立してon/offできる機能を提供する
2. WHERE Mode_1が有効化されている場合、THE MUP_Controller SHALL SMF-UPF間のPFCPトラフィックを盗聴する
3. WHERE Mode_2が有効化されている場合、THE MUP_Controller SHALL MMEからのGTPv2またはAMFからのNsmf_APIを受信する
4. THE MUP_Controller SHALL Mode_1とMode_2を同時に有効化できる
5. WHEN 起動時、THE MUP_Controller SHALL 設定ファイルから動作モードを読み込む

### 要件2: Mode_1によるPFCPトラフィック盗聴とセッション管理

**ユーザーストーリー:** ネットワークオペレーターとして、既存のSMF-UPF通信に影響を与えずにセッション情報を収集したい。これにより、既存システムを変更せずにMUP機能を追加できる。

#### 受入基準

1. WHEN Mode_1が有効化されている場合、THE MUP_Controller SHALL SMF-UPF間のPFCPパケットをキャプチャする
2. WHEN PFCPパケットを受信した場合、THE MUP_Controller SHALL パケットをDSLコンパイル済みDialect_Transformerで処理する
3. WHEN PFCP Session Establishment Requestを検出した場合、THE MUP_Controller SHALL 新規PFCP_Session_Stateを作成し、Session_InformationをIR_Managerに渡す
4. WHEN PFCP Session Modification Requestを検出した場合、THE MUP_Controller SHALL 既存PFCP_Session_Stateを更新し、更新されたSession_InformationをIR_Managerに渡す
5. WHEN PFCP Session Deletion Requestを検出した場合、THE MUP_Controller SHALL PFCP_Session_Stateを削除し、セッションIDをIR_Managerに渡して削除を指示する
6. THE MUP_Controller SHALL 盗聴したPFCPトラフィックをパススルーしてSMF-UPF間の通信に影響を与えない
7. THE PFCP_Session_State_Manager SHALL セッションのライフサイクル（Establishment → Modification → Deletion）を管理する
8. THE PFCP_Session_State_Manager SHALL Modificationメッセージの差分情報を既存ステートにマージする
9. WHEN EstablishmentなしでModificationを受信した場合、THE MUP_Controller SHALL エラーログを出力して処理をスキップする

### 要件3: Mode_2による既存SMF実装とのインテグレーション

**ユーザーストーリー:** ネットワークアーキテクトとして、既存のSMF実装（free5GC、Open5GS等）からセッション情報を取得したい。これにより、成熟したSMF実装を活用しながらMUP機能を追加できる。

#### 受入基準

1. WHERE Mode_2が有効化されている場合、THE MUP_Controller SHALL 既存SMF実装からSession_Informationを受信する
2. THE MUP_Controller SHALL 既存SMF実装に統合可能なSession_Information出力プラグインを提供する
3. THE プラグイン SHALL セッション確立時にSession_Informationを抽出してMUP_Controllerに送信する
4. THE プラグイン SHALL セッション更新時にSession_Informationを抽出してMUP_Controllerに送信する
5. THE プラグイン SHALL セッション削除時にSession_Informationを抽出してMUP_Controllerに送信する
6. THE MUP_Controller SHALL プラグインからのSession_Information受信にgRPCまたはHTTP APIを使用する
7. THE プラグイン SHALL free5GC、Open5GS、OAI等の主要なオープンソースSMF実装に対応する

### 要件4: Session Information（IR）によるデータモデル統一

**ユーザーストーリー:** 開発者として、PFCP方言やSMF実装の差異を吸収する統一的なセッション情報データモデルを使用したい。これにより、異なるベンダーの実装に対応しやすくなる。

#### 受入基準

1. THE MUP_Controller SHALL PFCP方言とSMF実装の差異を吸収するSession_Information（IR）データ構造を定義する
2. THE Session_Information SHALL セッション識別情報（SessionID、SEID）を含む
3. THE Session_Information SHALL UE情報（UEIPAddress、UEPrefix）を含む
4. THE Session_Information SHALL トンネル情報（TEID、QFI）を含む
5. THE Session_Information SHALL エンドポイント情報（EndpointAddress、NetworkInstance）を含む
6. THE IR_Manager SHALL Session_InformationとStatic_ContextからBGP_RIB_Infoを生成する
7. THE BGP_RIB_Info SHALL GoBGP MUP_SAFI RIB構成に必要な全ての情報を含む

### 要件5: PFCP方言対応のためのDSL（Mode_1専用）

**ユーザーストーリー:** 開発者として、新しいPFCP方言に対応する際に、人間可読な形式で変換ロジックを記述したい。これにより、保守性と拡張性を向上させる。

#### 受入基準

1. THE MUP_Controller SHALL PFCP方言とSession_Information間の変換を記述するDSLを提供する（Mode_1専用）
2. THE DSL SHALL 人間が読み書きしやすい構文を持つ
3. THE MUP_Controller SHALL DSLで記述された変換定義をGoコードにコンパイルする機能を提供する
4. WHEN DSLコンパイラを実行した場合、THE MUP_Controller SHALL Dialect_TransformerインターフェースのGoコード実装を生成する
5. THE 生成されたDialect_Transformer SHALL PFCPメッセージからPFCP_Session_State差分への変換を実行する
6. THE 生成されたDialect_Transformer SHALL PFCP_Session_StateからSession_Informationへの変換を実行する
7. THE DSL SHALL sample/Keysight/配下のPFCPキャプチャデータを参照実装として使用する
8. THE MUP_Controller SHALL DSL変換ロジックの正確性を自動テストする機能を提供する
9. THE Dialect_Transformer SHALL Dependency InjectionパターンでPFCP_Session_State_Managerに注入される

### 要件6: DSLのパーサー、プリティプリンター、Linter

**ユーザーストーリー:** 開発者として、DSL定義ファイルを正確に解析し、整形された形式で出力し、品質をチェックしたい。これにより、DSL定義の品質を保証する。

#### 受入基準

1. WHEN DSL定義ファイルが提供された場合、THE DSL_Parser SHALL ファイルを解析してAST（抽象構文木）を生成する
2. WHEN 不正なDSL定義ファイルが提供された場合、THE DSL_Parser SHALL 詳細なエラーメッセージを返す
3. THE DSL_Pretty_Printer SHALL ASTを整形されたDSL定義ファイルに変換する
4. FOR ALL 有効なDSL定義、DSL_ParserでパースしてDSL_Pretty_Printerで出力し再度DSL_Parserでパースした結果 SHALL 元のASTと等価である（ラウンドトリップ特性）
5. THE DSL_Linter SHALL DSL定義ファイルの構文チェックを実行する
6. THE DSL_Linter SHALL DSL定義のベストプラクティス違反を検出する
7. WHEN DSL_Linterが問題を検出した場合、THE DSL_Linter SHALL 問題の種類、位置、修正提案を含むレポートを出力する

### 要件7: 静的コンテクストの管理

**ユーザーストーリー:** システム管理者として、Network_InstanceとRD/RTのマッピングをJSON設定ファイルで管理したい。これにより、ネットワーク構成の変更に柔軟に対応できる。

#### 受入基準

1. THE MUP_Controller SHALL 静的コンテクストをJSON形式の設定ファイルから読み込む
2. THE 静的コンテクスト SHALL Network_InstanceからRDへのマッピングを含む
3. THE 静的コンテクスト SHALL Network_InstanceからRTへのマッピングを含む
4. THE 静的コンテクスト SHALL Type_1_Session_Transformed_Route用のSource_Address設定を含む（optional）
5. THE 静的コンテクスト SHALL Type_2_Session_Transformed_Route用のMUP_Extended_Community（Segment_Identifier）設定を含む
6. THE 静的コンテクスト SHALL Type_2_Session_Transformed_Route用のEndpoint_Address_Length設定を含む
7. THE 静的コンテクスト SHALL MUP_ControllerのNexthop_Address設定を含む
8. THE MUP_Controller SHALL JSON_Schemaを使用して設定ファイルを検証する
9. WHEN 設定ファイルがJSON_Schemaに適合しない場合、THE MUP_Controller SHALL 起動時にエラーを出力して終了する
10. WHEN 設定ファイルが更新された場合、THE MUP_Controller SHALL 再起動後に新しい設定を適用する

### 要件8: GoBGPへのMUP SAFI RIB出力

**ユーザーストーリー:** ネットワークオペレーターとして、収集したセッション情報をBGPルートとして配信したい。これにより、SRv6ネットワークでモバイルセッションのルーティングを実現できる。

#### 受入基準

1. THE MUP_Controller SHALL GoBGPとgRPCで通信する
2. WHEN IR_ManagerがSession_InformationとStatic_ContextからBGP_RIB_Infoを生成した場合、THE MUP_Controller SHALL Type_1_Session_Transformed_RouteまたはType_2_Session_Transformed_RouteをGoBGP RIBに追加する
3. WHEN Session_Informationが更新された場合、THE MUP_Controller SHALL 対応するGoBGP RIBエントリを更新する
4. WHEN Session_Informationが削除された場合、THE MUP_Controller SHALL 対応するGoBGP RIBエントリを削除する
5. THE BGP_RIB_Info SHALL Session_InformationとStatic_Context（RD、RT、MUP_Extended_Community、Source_Address、Endpoint_Address_Length、Nexthop_Address）の合成結果を含む
6. WHEN Type_1_Session_Transformed_Routeを生成する場合、THE MUP_Controller SHALL RD、RT、Source_Address（optional）、Nexthop_AddressをBGP_RIB_Infoから取得する
7. WHEN Type_2_Session_Transformed_Routeを生成する場合、THE MUP_Controller SHALL RD、RT、MUP_Extended_Community、Endpoint_Address_Length、Nexthop_AddressをBGP_RIB_Infoから取得する
8. THE MUP_Controller SHALL BGP_RIB_Infoから直接BGP RIBを生成できる
9. THE MUP_Controller SHALL https://github.com/osrg/gobgp/blob/master/docs/sources/srv6_mup.md に記載されたMUP SAFI仕様に準拠する

### 要件9: エラーハンドリングとログ出力

**ユーザーストーリー:** システム管理者として、システムの動作状況とエラーをログで確認したい。これにより、問題発生時の原因究明と事後解析を可能にする。

#### 受入基準

1. WHEN エラーが発生した場合、THE MUP_Controller SHALL エラー内容を構造化ログとして出力する
2. THE ログ SHALL タイムスタンプ、ログレベル、コンポーネント名、メッセージを含む
3. THE MUP_Controller SHALL ログレベル（DEBUG、INFO、WARN、ERROR）を設定ファイルで指定できる
4. WHEN PFCPパケットの解析に失敗した場合、THE MUP_Controller SHALL エラーログを出力して処理を継続する
5. WHEN GoBGPとの通信に失敗した場合、THE MUP_Controller SHALL エラーログを出力してリトライする
6. WHEN 致命的なエラーが発生した場合、THE MUP_Controller SHALL エラーログを出力して終了する

### 要件10: パフォーマンスとスケーラビリティ

**ユーザーストーリー:** ネットワークオペレーターとして、大規模なモバイルネットワークに対応できる性能を持つシステムを運用したい。

#### 受入基準

1. THE MUP_Controller SHALL 10,000セッションを同時に管理できる
2. WHEN Mode_1でPFCPパケットを受信した場合、THE MUP_Controller SHALL 100ミリ秒以内にIRに変換する
3. WHEN IRが更新された場合、THE MUP_Controller SHALL 200ミリ秒以内にGoBGP RIBを更新する
4. THE MUP_Controller SHALL メモリ使用量が2GB以下である（10,000セッション管理時）
5. THE MUP_Controller SHALL CPU使用率が50%以下である（通常動作時、4コアCPU想定）

### 要件11: デプロイメントと運用

**ユーザーストーリー:** システム管理者として、簡単にデプロイして運用できるシステムを使用したい。

#### 受入基準

1. THE MUP_Controller SHALL スタンドアロンバイナリとして実行できる
2. WHEN コマンドラインから起動した場合、THE MUP_Controller SHALL 設定ファイルのパスを引数で指定できる
3. THE MUP_Controller SHALL 設定ファイルが指定されない場合、デフォルトパス（./config.json）から読み込む
4. THE MUP_Controller SHALL バージョン情報を表示するコマンドラインオプション（--version）を提供する
5. THE MUP_Controller SHALL ヘルプメッセージを表示するコマンドラインオプション（--help）を提供する
6. THE MUP_Controller SHALL SIGTERM、SIGINTシグナルを受信した場合、グレースフルシャットダウンを実行する
7. THE MUP_Controller SHALL Mode_1で使用するPFCP方言を設定ファイルで指定できる
8. WHEN 指定された方言のDialect_Transformerが存在しない場合、THE MUP_Controller SHALL 起動時にエラーを出力して終了する

### 要件12: DSL変換ロジックのテスト可能性

**ユーザーストーリー:** 開発者として、DSL変換ロジックの正確性を複数のレベルで検証したい。これにより、PFCP方言対応の品質を保証できる。

#### 受入基準

**レベル1: Dialect Transformer単体テスト**

1. THE MUP_Controller SHALL Dialect_Transformerの単体テスト機能を提供する
2. THE 単体テスト SHALL 単一のPFCPメッセージからPFCP_Session_Stateへの変換を検証する
3. THE 単体テスト SHALL PFCP_Session_StateからSession_Informationへの変換を検証する
4. THE 単体テスト SHALL JSON形式のテストデータ（入力PFCPメッセージ、期待PFCP_Session_State）を読み込む
5. WHEN 単体テストが失敗した場合、THE MUP_Controller SHALL 期待値と実際の変換結果の差分を出力する

**レベル2: セッションライフサイクルテスト**

6. THE MUP_Controller SHALL セッションライフサイクル（Establishment → Modification → Deletion）の統合テスト機能を提供する
7. THE ライフサイクルテスト SHALL 一連のPFCPメッセージを順次処理し、各ステップでのSession_Informationを検証する
8. THE ライフサイクルテスト SHALL Modificationメッセージの差分マージが正しく動作することを検証する
9. THE ライフサイクルテスト SHALL JSON形式のテストデータ（PFCPメッセージ列、期待Session_Information列）を読み込む
10. WHEN ライフサイクルテストが失敗した場合、THE MUP_Controller SHALL どのステップで失敗したかと差分を出力する

**レベル3: PCAP統合テスト**

11. THE MUP_Controller SHALL PFCPキャプチャデータ（PCAP形式）からの統合テスト機能を提供する
12. THE PCAP統合テスト SHALL PCAPファイルからPFCPメッセージを抽出する
13. THE PCAP統合テスト SHALL 抽出したPFCPメッセージをDialect_TransformerとPFCP_Session_State_Managerで処理する
14. THE PCAP統合テスト SHALL 最終的に生成されたSession_Information列をJSON期待値と比較検証する
15. THE PCAP統合テスト SHALL sample/Keysight/配下のPFCPキャプチャデータを使用できる
16. WHEN PCAP統合テストが失敗した場合、THE MUP_Controller SHALL どのメッセージで失敗したかと差分を出力する

**共通要件**

17. THE 全テスト SHALL 各PFCP方言に対して独立して実行できる
18. THE 全テスト SHALL CI/CDパイプラインに統合可能である
19. THE 全テスト SHALL 自動実行可能なコマンドラインツールとして提供される

### 要件13: スコープ外機能の明確化

**ユーザーストーリー:** プロジェクトマネージャーとして、本プロジェクトのスコープを明確にしたい。

#### 受入基準

1. THE MUP_Controller SHALL ISD（Interworking Segment Discovery）機能を実装しない
2. THE MUP_Controller SHALL DSD（Direct Segment Discovery）機能を実装しない
3. THE MUP_Controller SHALL SMF機能を実装しない（既存SMF実装を活用する）
4. THE MUP_Controller SHALL GTPv2/Nsmf_APIの直接処理を実装しない（既存SMF実装が処理する）
5. THE MUP_Controller SHALL UE_IP割り当て、UPF選択、QoS管理等のSMF機能を実装しない（既存SMF実装が処理する）
6. THE MUP_Controller SHALL 初期バージョンでPFCP方言の自動検出機能を実装しない（将来実装として考慮）

## 非機能要件

### 可用性

- THE MUP_Controller SHALL 24時間365日の連続稼働に耐える設計とする
- WHEN クラッシュした場合、THE MUP_Controller SHALL 再起動後に状態を復元できる（永続化機能は将来実装）

### 拡張性

- THE DSL SHALL 新しいPFCP方言の追加を容易にする設計とする
- THE Dialect_Transformer SHALL Dependency Injectionパターンにより実行時に切り替え可能とする
- THE システム SHALL 将来的にPFCP方言の自動検出機能を追加可能な設計とする

### 保守性

- THE MUP_Controller SHALL Goの標準的なコーディング規約に従う
- THE MUP_Controller SHALL 単体テストカバレッジ80%以上を達成する
- THE DSL SHALL 変換ロジックのドキュメントとして機能する

### セキュリティ

- THE MUP_Controller SHALL 入力データ（PFCP、GTPv2、Nsmf_API）の検証を行う
- THE MUP_Controller SHALL 不正な入力に対してシステムをクラッシュさせない
- THE MUP_Controller SHALL 機密情報（セッション情報）をログに出力しない（DEBUG レベルを除く）

## 制約事項

1. ISD（Interworking Segment Discovery）機能は本プロジェクトのスコープ外
2. DSD（Direct Segment Discovery）機能は本プロジェクトのスコープ外
3. SMF機能の実装は本プロジェクトのスコープ外（既存SMF実装を活用する）
4. Mode_2では既存SMF実装（free5GC、Open5GS、OAI等）とのインテグレーションが必要
5. 初期バージョンはスタンドアロンバイナリとして提供し、コンテナ化は将来対応
6. セッション状態の永続化は将来実装（初期バージョンはインメモリのみ）
