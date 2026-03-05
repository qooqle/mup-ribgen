dialect "Keysight_N9"
version "1.0"

// Keysight N9 PFCP Dialect Transformer
//
// This DSL defines field mappings for the Keysight Network Tester N9-interface
// PFCP sessions captured via passive sniffer (Mode 1).
//
// Session flow observed in sample/Keysight/pfcp-n9.pcap:
//   [msg=50] Session Establishment Request  → creates state with SMF SEID
//   [msg=52] Session Modification Request   → delivers PDR/FAR/QER with UE IP, TEID, QFI
//   [msg=54] Session Deletion Request       → tears down session
//
// Parser field layout (pkg/pfcp/parser.go):
//   req.Fields["pfcp"]["seid"]             -- SEID from S-flag header
//   req.Fields["pfcp"]["f_seid"]["seid"]   -- F-SEID IE: CP SEID (SMF side)
//   req.Fields["pfcp"]["f_seid"]["ipv4"]   -- F-SEID: CP IPv4 address
//   req.Fields["pfcp"]["create_pdr"]       -- []map: Create PDR IEs
//     [*]["pdr_id"]                        -- PDR ID
//     [*]["pdi"]["source_interface"]       -- 0=Access, 1=Core
//     [*]["pdi"]["network_instance"]       -- Network Instance string
//     [*]["pdi"]["ue_ip_address"]["ipv4"]  -- UE IP (DL PDR, source_interface=1)
//     [*]["pdi"]["f_teid"]["teid"]         -- F-TEID TEID value
//     [*]["far_id"]                        -- referenced FAR ID
//     [*]["qer_id"]                        -- referenced QER ID (via pdi)
//   req.Fields["pfcp"]["create_far"]       -- []map: Create FAR IEs
//     [*]["far_id"]                        -- FAR ID
//     [*]["forwarding_parameters"]["outer_header_creation"]["teid"]  -- N9 TEID
//     [*]["forwarding_parameters"]["outer_header_creation"]["ipv4"]  -- N9 endpoint IP
//     [*]["forwarding_parameters"]["network_instance"]               -- N9 network name
//   req.Fields["pfcp"]["create_qer"]       -- []map: Create QER IEs
//     [*]["qer_id"]                        -- QER ID

// --- Mapping 1: Establishment Request → Session State -----------------------
//
// The Establishment (msg_type=50) carries the SMF F-SEID IE (IE type 57).
// The SEID in the F-SEID is the SMF (CP) SEID; the UPF SEID is assigned later.
mapping pfcp_establishment_to_state {
    // SMF F-SEID → state.SEID
    pfcp.f_seid.seid -> SEID
}

// --- Mapping 2: Modification Request → Session State Delta ------------------
//
// The Modification (msg_type=52) delivers Create PDR / Create FAR / Create QER IEs.
// The UE IP address and N9 tunnel endpoint appear in this message.
// Note: The SEID in the request header is the UPF-assigned SEID (from response msg=51).
mapping pfcp_modification_to_delta {
    // UPF SEID from request header
    pfcp.seid -> SEID
    // PDR ID from each Create PDR IE
    pfcp.create_pdr[*].pdr_id -> UpdatePDRs
    // FAR ID from each Create FAR IE
    pfcp.create_far[*].far_id -> UpdateFARs
}

// --- Mapping 3: Session State → Session Information (IR) --------------------
//
// After establishment + modification, the state holds PDR/FAR/QER entries.
// The SessionInformation is derived from the accumulated state:
//   - SEID:            state.SEID (from SMF F-SEID)
//   - UEIPAddress:     DL PDR (source_interface=Core) PDI UE IP Address
//   - NetworkInstance: N9 FAR forwarding_parameters.network_instance
//   - EndpointAddress: N9 FAR outer_header_creation.ipv4
//   - TEID:            N9 FAR outer_header_creation.teid
//   - QFI:             QER qfi_value
//
// NOTE: Complex extraction from PDR/FAR maps (typed Go structs) requires
// manual enhancement of the generated transformer code (see Task 4.8).
mapping state_to_session_info {
    // SEID is directly available from the state struct
    SEID -> SEID
}
