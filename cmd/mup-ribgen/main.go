// cmd/mup-ribgen is the MUP Controller entry point for Mode 1
// (passive PFCP sniffing → BGP MUP SAFI route generation).
//
// Usage:
//
//	# dry-run from PCAP file (no GoBGP needed):
//	mup-ribgen --pcap sample/Keysight/pfcp-n9.pcap \
//	           --static-context static_context.json \
//	           --dialect keysight_n9 --dry-run
//
//	# live sniffing + send to GoBGP:
//	mup-ribgen --interface eth0 \
//	           --static-context static_context.json \
//	           --dialect keysight_n9 \
//	           --gobgp-addr 127.0.0.1:50051 --route-type type1
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	// blank import to register all compiled dialect transformers
	_ "github.com/qooqle/mup-ribgen/pkg/dialect"

	"github.com/qooqle/mup-ribgen/pkg/bgp"
	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/mode1"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
	"github.com/qooqle/mup-ribgen/pkg/pipeline"
	"github.com/qooqle/mup-ribgen/pkg/staticctx"
)

func main() {
	// --- flags ---------------------------------------------------------------
	var (
		flagPCAP          = flag.String("pcap", "", "PCAP file to replay (Mode 1, no live capture)")
		flagInterface     = flag.String("interface", "", "network interface to sniff (live Mode 1)")
		flagStaticCtx     = flag.String("static-context", "static_context.json", "static context JSON config")
		flagDialect       = flag.String("dialect", "Keysight_N9", "PFCP dialect transformer name")
		flagDryRun        = flag.Bool("dry-run", false, "print BGP routes instead of sending to GoBGP")
		flagGoBGPAddr     = flag.String("gobgp-addr", "127.0.0.1:50051", "GoBGP daemon gRPC address (host:port)")
		flagRouteType     = flag.String("route-type", "type1", "MUP SAFI route type: type1 or type2")
		flagLogLevel      = flag.String("log-level", "info", "log level: debug, info, warn, error")
		flagVersion       = flag.Bool("version", false, "print version and exit")
		flagChannelBuffer = flag.Int("channel-buffer", 128, "event channel buffer size")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println("mup-ribgen version dev")
		os.Exit(0)
	}

	// --- logger --------------------------------------------------------------
	setupLogger(*flagLogLevel)

	// --- validate flags ------------------------------------------------------
	if *flagPCAP == "" && *flagInterface == "" {
		slog.Error("either --pcap or --interface must be specified")
		flag.Usage()
		os.Exit(1)
	}
	if *flagPCAP != "" && *flagInterface != "" {
		slog.Error("--pcap and --interface are mutually exclusive")
		os.Exit(1)
	}

	// --- static context ------------------------------------------------------
	sctxMgr := staticctx.New()
	if err := sctxMgr.Load(*flagStaticCtx); err != nil {
		slog.Error("failed to load static context", "path", *flagStaticCtx, "err", err)
		os.Exit(1)
	}
	slog.Info("static context loaded", "path", *flagStaticCtx)

	// --- dialect transformer -------------------------------------------------
	transformer, err := dialect.Lookup(*flagDialect)
	if err != nil {
		slog.Error("dialect not found", "dialect", *flagDialect,
			"available", dialect.Registered())
		os.Exit(1)
	}
	slog.Info("dialect loaded", "dialect", *flagDialect)

	// --- sniffer -------------------------------------------------------------
	var sniffer pfcp.Sniffer
	if *flagPCAP != "" {
		slog.Info("mode: PCAP replay", "file", *flagPCAP)
		sniffer = pfcp.NewPCAPFileSniffer(*flagPCAP)
	} else {
		slog.Info("mode: live capture", "interface", *flagInterface)
		sniffer = pfcp.NewSniffer(*flagInterface)
	}

	// --- context & signal handling -------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// --- mode 1 controller ---------------------------------------------------
	ctrl := mode1.New(
		mode1.Config{Interface: *flagInterface, ChannelBuffer: *flagChannelBuffer},
		sniffer,
		transformer,
	)

	// --- IR manager ----------------------------------------------------------
	irMgr := ir.NewManager(sctxMgr, *flagChannelBuffer)

	// --- BGP sender ----------------------------------------------------------
	var sender pipeline.BGPSender
	if *flagDryRun {
		slog.Info("dry-run mode: routes will be printed to stdout")
		sender = &dryRunSender{routeType: *flagRouteType}
	} else {
		host, port := parseAddr(*flagGoBGPAddr)
		client := bgp.NewClient(bgp.Config{Address: host, Port: port})
		if err := client.Connect(ctx); err != nil {
			slog.Error("cannot connect to GoBGP", "addr", *flagGoBGPAddr, "err", err)
			os.Exit(1)
		}
		defer client.Close()
		slog.Info("connected to GoBGP", "addr", *flagGoBGPAddr)
		sender = client
	}

	// --- wire up pipeline ----------------------------------------------------
	pipeline.ConnectMode1ToIR(ctx, ctrl, irMgr)
	pipeline.ConnectIRToBGP(ctx, irMgr, sender, *flagRouteType)

	// --- start ---------------------------------------------------------------
	if err := ctrl.Start(ctx); err != nil {
		slog.Error("failed to start Mode 1 controller", "err", err)
		os.Exit(1)
	}

	slog.Info("mup-ribgen running, press Ctrl+C to stop")

	// Block until context cancelled (signal received) or PCAP replay done.
	// For PCAP replay the Events channel closes naturally; we wait on ctx.
	<-ctx.Done()
	slog.Info("shutting down")
	ctrl.Stop()
}

// --- dry-run sender ----------------------------------------------------------

type dryRunSender struct {
	routeType string
}

func (d *dryRunSender) AddType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("ADD", "type1", rib)
}
func (d *dryRunSender) AddType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("ADD", "type2", rib)
}
func (d *dryRunSender) UpdateType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("UPDATE", "type1", rib)
}
func (d *dryRunSender) UpdateType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("UPDATE", "type2", rib)
}
func (d *dryRunSender) DeleteType1Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("DELETE", "type1", rib)
}
func (d *dryRunSender) DeleteType2Route(_ context.Context, rib *ir.BGPRIBInfo) error {
	return d.print("DELETE", "type2", rib)
}

type routeOutput struct {
	Op        string   `json:"op"`
	RouteType string   `json:"route_type"`
	SEID      uint64   `json:"seid"`
	UEID      string   `json:"ue_ip,omitempty"`
	UEPrefix  string   `json:"ue_prefix,omitempty"`
	Endpoint  string   `json:"endpoint"`
	TEID      uint32   `json:"teid"`
	QFI       uint8    `json:"qfi,omitempty"`
	RD        string   `json:"rd"`
	RT        []string `json:"rt"`
	Nexthop   string   `json:"nexthop"`
	Network   string   `json:"network_instance"`
}

func (d *dryRunSender) print(op, routeType string, rib *ir.BGPRIBInfo) error {
	out := routeOutput{
		Op:        op,
		RouteType: routeType,
		SEID:      rib.SEID,
		UEID:      rib.UEIPAddress,
		UEPrefix:  rib.UEPrefix,
		Endpoint:  rib.EndpointAddress,
		TEID:      rib.TEID,
		QFI:       rib.QFI,
		RD:        rib.RD,
		RT:        rib.RT,
		Nexthop:   rib.NexthopAddress,
		Network:   rib.NetworkInstance,
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return nil
}

// --- helpers -----------------------------------------------------------------

func setupLogger(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
}

func parseAddr(addr string) (host string, port int) {
	host = "127.0.0.1"
	port = 50051
	fmt.Sscanf(addr, "%s", &addr)
	var h string
	var p int
	if n, _ := fmt.Sscanf(addr, "%[^:]:%d", &h, &p); n == 2 {
		host = h
		port = p
	}
	return
}
