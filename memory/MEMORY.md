# mup-ribgen Project Memory

## Project Overview
SRv6 MUP Controller in Go. Passive PFCP sniffing (Mode 1) + SMF integration (Mode 2).
Converts mobile session info to BGP MUP SAFI routes via GoBGP gRPC.

## Key Architecture
- `pkg/ir` – SessionInformation, BGPRIBInfo, StaticContext data models
- `pkg/pfcp` – Sniffer, Parser, SessionManager, types
- `pkg/dialect` – DialectTransformer interface + registry + Keysight_N9 transformer
- `pkg/dsl` – DSL lexer/parser/compiler/linter/pretty-printer
- `pkg/dslruntime` – RT helper (GetField, GetArray, CoerceUint64/32/16/8/String, transforms)
- `pkg/testharness` – 3-level harness (unit/lifecycle/PCAP)
- `pkg/mode1` – Mode 1 Controller (Sniffer→Parser→SessionManager pipeline)
- `pkg/staticctx`, `pkg/config`, `pkg/logger` – foundation packages
- `cmd/dslc` – DSL compiler CLI

## Phase Progress (tasks.md = .kiro/specs/srv6-mup-controller/tasks.md)
- Phase 1 (Tasks 1.1-1.9): DONE – IR models, StaticContextManager, Logger, Config
- Phase 2 (Tasks 2.1-2.13): DONE – DSL Lexer/Parser/Compiler/Linter/Pretty-printer, 3-level TestHarness, 28 properties
- Phase 3 (Tasks 4.1-4.11): DONE – PFCP Sniffer/Parser/SessionManager, Keysight_N9 DSL+transformer, test data, harness tests, Mode1 Controller
- Phase 4 (Tasks 5.1-5.9): DONE – IR Manager, GoBGP gRPC client, Type1/2 route generation, pipeline connector, e2e tests

## Critical Implementation Notes

### PFCP apply_action encoding (3GPP TS 29.244 §8.2.26)
- The Apply Action IE is 2 octets (big-endian uint16)
- FORW = bit 2 of Octet 5 (high byte) = **0x0200** in uint16
- BUFF = bit 3 of Octet 5 (high byte) = **0x0400** in uint16
- Binary parser returns these correctly; test JSON data must use 512/1024, NOT 2/4

### PFCP SEID mapping (PFCP protocol §7.2.2)
- Establishment Request: S-flag=0, no header SEID; CP F-SEID IE carries CP's SEID
- Establishment Response: header SEID = CP's SEID; UP F-SEID IE carries UP's SEID
- Modification/Deletion Request: header SEID = UP's SEID
- SessionManager stores sessions under state.SEID (CP's SEID from F-SEID IE)
- Level3 RunLevel3 processes EstablishmentResponse (msg=51) to register upSEID→cpSEID alias
- inMemoryStateManager.seidAliases[upSEID] = cpSEID for modification lookup

### DSL Compiler limitations
- EstablishmentToState / ModificationToState: generated code works (map source)
- StateToSessionInfo: must be written manually (struct source not supported by compiler)
- Generated loop rule uses rt.GetField for array access (not direct map indexing)
- Coerce functions needed for type safety: CoerceUint64/32/16/8 handle float64 from JSON

### Test data apply_action values
- test_data/keysight_n9/unit/modification.json: apply_action=512 (FORW), 1024 (BUFF)
- test_data/keysight_n9/lifecycle/n9_session.json: same, modification.SEID=1 (state.SEID)
- sample/Keysight/pfcp-n9-expected.json: 4 entries (1 est + 3 mods from PCAP)

### Build tags
- Default (no tags): StubSniffer, no libpcap needed
- `-tags pcap`: real gopacket/pcap sniffer, requires `brew install libpcap`

## Key File Paths
- DSL definition: `dsl/keysight_n9.dsl`
- Transformer: `pkg/dialect/keysight_n9_transformer.go`
- Harness tests: `pkg/dialect/keysight_n9_harness_test.go`
- Test data: `test_data/keysight_n9/unit/`, `test_data/keysight_n9/lifecycle/`
- PCAP sample: `sample/Keysight/pfcp-n9.pcap` + expected: `sample/Keysight/pfcp-n9-expected.json`
- Mode1 controller: `pkg/mode1/controller.go`

## Go Module
- Module: `github.com/qooqle/mup-ribgen`
- Key deps: gopacket v1.1.19, gopter v0.2.11, gojsonschema v1.2.0, gobgp/v3 v3.37.0, grpc v1.79.1
- Runtime: mise (go=1.25.0, python=3.12) — upgraded from 1.23 when adding gRPC
- Test runner: `mise exec -- go test ./...`

## Phase 4 Architecture (Task 5)
- `pkg/ir/manager.go` – IR Manager: SessionInfo + StaticCtx → BGPRIBInfo, emits BGPEvents
- `pkg/bgp/client.go` – GoBGP gRPC client: Connect/AddType1/2/Update/Delete route methods
- `pkg/bgp/routes.go` – BuildType1Route/BuildType2Route: anypb.Any MUP NLRI construction
- `pkg/pipeline/pipeline.go` – ConnectMode1ToIR, ConnectIRToBGP goroutine connectors

## GoBGP API Notes (gobgp/v3 v3.37.0)
- `MUPType1SessionTransformedRoute.Rd` is `*anypb.Any` (not *RouteDistinguisher directly)
- Use `anypb.New(&gobgpapi.RouteDistinguisherTwoOctetASN{Admin: asn, Assigned: admin})`
- `TwoOctetAsSpecificExtended` field is `Asn` (not `As`)
- MUP SAFI: Family_SAFI_MUP, AFI_IP or AFI_IP6
- `MUPExtended` uses `SegmentId2` (uint32) and `SegmentId4` (uint32)
