// Package pipeline connects the Mode-1 Controller → IR Manager → BGP Client
// pipeline (req 2.3, 2.4, 2.5, 8.2, 8.3, 8.4).
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/mode1"
)

// BGPSender is implemented by bgp.Client and routes BGPRIBInfo to GoBGP.
type BGPSender interface {
	AddType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error
	AddType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error
	UpdateType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error
	UpdateType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error
	DeleteType1Route(ctx context.Context, rib *ir.BGPRIBInfo) error
	DeleteType2Route(ctx context.Context, rib *ir.BGPRIBInfo) error
}

// ConnectMode1ToIR reads SessionEvents from the Mode-1 Controller and drives
// the IR Manager (req 2.3, 2.4, 2.5). It returns immediately and runs a
// goroutine that exits when ctrl.Events() is closed (i.e. ctrl.Stop() called).
func ConnectMode1ToIR(ctx context.Context, ctrl *mode1.Controller, irMgr *ir.Manager) {
	go func() {
		for ev := range ctrl.Events() {
			switch ev.Type {
			case "establishment":
				if ev.Info == nil {
					continue
				}
				if err := irMgr.HandleCreate(ev.Info); err != nil {
					// NetworkInstance may be empty right after establishment
					// (before the first Modification populates PDRs/FARs).
					// This is expected; the entry will be created on modification.
					slog.Debug("pipeline: IR create skipped (no static ctx yet)",
						"seid", ev.SEID, "err", err)
				}
			case "modification":
				if ev.Info == nil {
					continue
				}
				if err := irMgr.HandleUpdate(ev.Info); err != nil {
					slog.Warn("pipeline: IR update failed",
						"seid", ev.SEID, "err", err)
				}
			case "deletion":
				irMgr.HandleDelete(ev.SEID)
			}
		}
	}()
}

// ConnectIRToBGP reads BGPEvents from the IR Manager and sends the corresponding
// MUP SAFI routes to GoBGP via sender (req 8.2, 8.3, 8.4).
// RouteType selects which MUP route type to use ("type1" or "type2").
// Runs asynchronously; stops when irMgr.Events() is drained (no more events).
func ConnectIRToBGP(ctx context.Context, irMgr *ir.Manager, sender BGPSender, routeType string) {
	go func() {
		types := normalizeRouteTypes(routeType)
		lastSent := make(map[string]string)
		for ev := range irMgr.Events() {
			if ev.Info == nil && ev.Type != ir.BGPEventDelete {
				continue
			}
			switch ev.Type {
			case ir.BGPEventCreate:
				for _, rt := range types {
					if !isRIBReadyForRouteType(rt, ev.Info) {
						slog.Warn("pipeline: skip BGP add due to incomplete RIB",
							"route_type", rt, "route_key", ev.RouteKey, "seid", ev.SEID)
						continue
					}
					logBGPEvent("add", rt, ev)
					sendBGP(ctx, sender, rt, "add", ev.Info)
					lastSent[routeStateKey(rt, ev.Info)] = ribFingerprint(rt, ev.Info)
				}
			case ir.BGPEventUpdate:
				for _, rt := range types {
					if !isRIBReadyForRouteType(rt, ev.Info) {
						slog.Warn("pipeline: skip BGP update due to incomplete RIB",
							"route_type", rt, "route_key", ev.RouteKey, "seid", ev.SEID)
						continue
					}
					k := routeStateKey(rt, ev.Info)
					fp := ribFingerprint(rt, ev.Info)
					if prev, ok := lastSent[k]; ok && prev == fp {
						slog.Debug("pipeline: suppress no-op BGP update",
							"route_type", rt, "route_key", ev.RouteKey, "seid", ev.SEID)
						continue
					}
					logBGPEvent("update", rt, ev)
					sendBGP(ctx, sender, rt, "update", ev.Info)
					lastSent[k] = fp
				}
			case ir.BGPEventDelete:
				if ev.Info != nil {
					for _, rt := range types {
						logBGPEvent("delete", rt, ev)
						sendBGP(ctx, sender, rt, "delete", ev.Info)
						delete(lastSent, routeStateKey(rt, ev.Info))
					}
				}
			}
		}
	}()
}

func normalizeRouteTypes(routeType string) []string {
	switch routeType {
	case "both":
		return []string{"type1", "type2"}
	case "type2":
		return []string{"type2"}
	default:
		return []string{"type1"}
	}
}

// sendBGP dispatches a BGP operation to the appropriate route type handler.
func sendBGP(ctx context.Context, sender BGPSender, routeType, op string, rib *ir.BGPRIBInfo) {
	var err error
	switch routeType {
	case "type2":
		switch op {
		case "add":
			err = sender.AddType2Route(ctx, rib)
		case "update":
			err = sender.UpdateType2Route(ctx, rib)
		case "delete":
			err = sender.DeleteType2Route(ctx, rib)
		}
	default: // "type1"
		switch op {
		case "add":
			err = sender.AddType1Route(ctx, rib)
		case "update":
			err = sender.UpdateType1Route(ctx, rib)
		case "delete":
			err = sender.DeleteType1Route(ctx, rib)
		}
	}
	if err != nil {
		slog.Warn("pipeline: BGP send failed",
			"op", op, "route_type", routeType, "route_key", rib.RouteKey, "seid", rib.SEID, "err", err)
	}
}

func logBGPEvent(op, routeType string, ev *ir.BGPEvent) {
	if ev == nil || ev.Info == nil {
		return
	}
	slog.Info("pipeline: BGP event",
		"op", op,
		"route_type", routeType,
		"route_key", ev.RouteKey,
		"seid", ev.SEID,
		"far_id", ev.Info.FARID,
		"network_instance", ev.Info.NetworkInstance,
		"endpoint", ev.Info.EndpointAddress,
		"teid", ev.Info.TEID,
		"qfi", ev.Info.QFI,
	)
}

func routeStateKey(routeType string, rib *ir.BGPRIBInfo) string {
	return routeType + "|" + rib.RouteKey
}

func isRIBReadyForRouteType(routeType string, rib *ir.BGPRIBInfo) bool {
	if rib == nil {
		return false
	}
	if rib.RD == "" || len(rib.RT) == 0 || rib.NexthopAddress == "" {
		return false
	}
	if rib.EndpointAddress == "" || rib.TEID == 0 {
		return false
	}
	switch routeType {
	case "type2":
		return rib.EndpointAddressLength != nil && *rib.EndpointAddressLength > 0
	default: // type1
		return rib.UEPrefix != "" || rib.UEIPAddress != ""
	}
}

func ribFingerprint(routeType string, rib *ir.BGPRIBInfo) string {
	base := fmt.Sprintf("%s|%d|%s|%s|%d|%s",
		rib.RD, rib.SEID, rib.EndpointAddress, rib.NexthopAddress, rib.TEID, strings.Join(rib.RT, ","))
	switch routeType {
	case "type2":
		epLen := 0
		if rib.EndpointAddressLength != nil {
			epLen = *rib.EndpointAddressLength
		}
		seg := ""
		if rib.MUPExtendedCommunity != nil {
			seg = fmt.Sprintf("%x", rib.MUPExtendedCommunity.SegmentIdentifier)
		}
		return fmt.Sprintf("%s|%d|%s", base, epLen, seg)
	default: // type1
		ue := rib.UEIPAddress
		if rib.UEPrefix != "" {
			ue = rib.UEPrefix
		}
		src := ""
		if rib.SourceAddress != nil {
			src = *rib.SourceAddress
		}
		return fmt.Sprintf("%s|%s|%d|%s", base, ue, rib.QFI, src)
	}
}
