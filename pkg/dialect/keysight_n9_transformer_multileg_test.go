package dialect

import (
	"testing"

	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

func TestKeysightN9_StateToSessionInfos_MultiLeg(t *testing.T) {
	tr := &KeysightN9Transformer{}
	state := &pfcp.PFCPSessionState{
		SEID: 1,
		PDRs: map[uint16]*pfcp.PDR{
			12: {
				PDRID: 12,
				Fields: map[string]interface{}{
					"far_id": uint32(12),
					"qer_id": uint32(6),
					"pdi": map[string]interface{}{
						"source_interface": uint8(1),
						"network_instance": "n9-nw",
						"ue_ip_address": map[string]interface{}{
							"ipv4": "172.16.0.1",
						},
					},
				},
			},
			11: {
				PDRID: 11,
				Fields: map[string]interface{}{
					"far_id": uint32(11),
					"qer_id": uint32(6),
				},
			},
		},
		FARs: map[uint32]*pfcp.FAR{
			11: {
				FARID: 11,
				Fields: map[string]interface{}{
					"apply_action": uint16(0x0200),
					"forwarding_parameters": map[string]interface{}{
						"network_instance": "n9-nw",
						"outer_header_creation": map[string]interface{}{
							"teid": uint32(1),
							"ipv4": "20.0.90.11",
						},
					},
				},
			},
			12: {
				FARID: 12,
				Fields: map[string]interface{}{
					"apply_action": uint16(0x0200),
					"forwarding_parameters": map[string]interface{}{
						"network_instance": "n3-nw",
						"outer_header_creation": map[string]interface{}{
							"teid": uint32(1),
							"ipv4": "20.0.3.10",
						},
					},
				},
			},
		},
		QERs: map[uint32]*pfcp.QER{
			6: {
				QERID: 6,
				Fields: map[string]interface{}{
					"qfi_value": uint8(5),
				},
			},
		},
	}

	infos, err := tr.StateToSessionInfos(state)
	if err != nil {
		t.Fatalf("StateToSessionInfos: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	if infos[0].EndpointAddress != "20.0.90.11" || infos[0].NetworkInstance != "n9-nw" {
		t.Fatalf("unexpected first leg: %+v", infos[0])
	}
	if infos[1].EndpointAddress != "20.0.3.10" || infos[1].NetworkInstance != "n3-nw" {
		t.Fatalf("unexpected second leg: %+v", infos[1])
	}
	if infos[0].QFI != 5 || infos[1].QFI != 5 {
		t.Fatalf("expected QFI=5 for both legs, got %d and %d", infos[0].QFI, infos[1].QFI)
	}
}

func TestKeysightN9_StateToSessionInfo_Compatibility(t *testing.T) {
	tr := &KeysightN9Transformer{}
	state := &pfcp.PFCPSessionState{
		SEID: 1,
		PDRs: map[uint16]*pfcp.PDR{
			12: {
				PDRID: 12,
				Fields: map[string]interface{}{
					"far_id": uint32(12),
					"qer_id": uint32(6),
					"pdi": map[string]interface{}{
						"source_interface": uint8(1),
						"network_instance": "n9-nw",
						"ue_ip_address": map[string]interface{}{
							"ipv4": "172.16.0.1",
						},
					},
				},
			},
		},
		FARs: map[uint32]*pfcp.FAR{
			12: {
				FARID: 12,
				Fields: map[string]interface{}{
					"apply_action": uint16(0x0200),
					"forwarding_parameters": map[string]interface{}{
						"network_instance": "n3-nw",
						"outer_header_creation": map[string]interface{}{
							"teid": uint32(1),
							"ipv4": "20.0.3.10",
						},
					},
				},
			},
		},
		QERs: map[uint32]*pfcp.QER{
			6: {QERID: 6, Fields: map[string]interface{}{"qfi_value": uint8(5)}},
		},
	}
	info, err := tr.StateToSessionInfo(state)
	if err != nil {
		t.Fatalf("StateToSessionInfo: %v", err)
	}
	if info.EndpointAddress != "20.0.3.10" || info.NetworkInstance != "n3-nw" || info.QFI != 5 {
		t.Fatalf("unexpected info: %+v", info)
	}
}

