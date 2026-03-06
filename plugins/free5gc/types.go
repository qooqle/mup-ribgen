// Package free5gc provides a mup-ribgen integration plugin for the free5GC SMF.
//
// # Integration guide
//
// 1. Add mup-ribgen as a dependency in your free5GC SMF fork:
//
//	go get github.com/qooqle/mup-ribgen
//
// 2. Initialize the MUPClient at SMF startup (e.g., in smf/main.go):
//
//	mupc, err := free5gc.NewMUPClient("127.0.0.1:9182", slog.Default())
//	if err != nil { ... }
//	defer mupc.Close()
//
// 3. Call the hook methods from the appropriate SMF context handlers:
//
//	// In your session establishment handler:
//	mupc.OnSessionEstablished(free5gc.SessionDataFromSMContext(smCtx))
//
//	// In your session modification handler:
//	mupc.OnSessionModified(free5gc.SessionDataFromSMContext(smCtx))
//
//	// In your session deletion handler:
//	mupc.OnSessionDeleted(free5gc.SessionDataFromSMContext(smCtx))
//
// # 4G/5G interworking
//
// For 4G sessions (GTPv2 S5-C from SGW-C), free5GC SMF handles the protocol
// translation internally. The plugin hooks into the resulting session context,
// which has the same structure regardless of whether the session originated
// from a 5G AMF (Nsmf REST) or a 4G SGW-C (GTPv2). For 4G sessions, QFI
// is typically 0.
package free5gc

// SessionData carries the session data extracted from a free5GC SMF context
// (smf/context.SMContext). Populate this from free5GC's internal structures.
type SessionData struct {
	// SessionID is the PDU session identifier (e.g., fmt.Sprintf("%d", smCtx.PDUSessionID)).
	SessionID string

	// SEID is the CP F-SEID assigned to this session.
	SEID uint64

	// UEIPAddress is the UE's IP address as a string (v4 or v6).
	UEIPAddress string

	// UEPrefix is the UE's IP prefix in CIDR notation (e.g., "10.0.0.1/32").
	UEPrefix string

	// TEID is the uplink GTP-U TEID on the N3 interface.
	TEID uint32

	// QFI is the QoS Flow Identifier (5G). Set to 0 for 4G sessions.
	QFI uint8

	// EndpointAddress is the UPF's N3 (outer) IP address.
	EndpointAddress string

	// NetworkInstance is the DNN (Data Network Name) / APN.
	// Used to look up the Static Context in mup-ribgen.
	NetworkInstance string
}
