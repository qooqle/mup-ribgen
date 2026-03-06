// Package pipeline connects the Mode-1 Controller → IR Manager → BGP Client
// pipeline (req 2.3, 2.4, 2.5, 8.2, 8.3, 8.4).
package pipeline

import (
	"context"
	"log/slog"

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
		for ev := range irMgr.Events() {
			if ev.Info == nil && ev.Type != ir.BGPEventDelete {
				continue
			}
			switch ev.Type {
			case ir.BGPEventCreate:
				for _, rt := range types {
					logBGPEvent("add", rt, ev)
					sendBGP(ctx, sender, rt, "add", ev.Info)
				}
			case ir.BGPEventUpdate:
				for _, rt := range types {
					logBGPEvent("update", rt, ev)
					sendBGP(ctx, sender, rt, "update", ev.Info)
				}
			case ir.BGPEventDelete:
				if ev.Info != nil {
					for _, rt := range types {
						logBGPEvent("delete", rt, ev)
						sendBGP(ctx, sender, rt, "delete", ev.Info)
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
