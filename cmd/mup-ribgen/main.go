// cmd/mup-ribgen is the MUP Controller entry point for Mode 1
// (passive PFCP sniffing → BGP MUP SAFI route generation).
//
// Usage:
//
//	# dry-run from PCAP file (no GoBGP needed):
//	mup-ribgen --pcap sample/Keysight/pfcp-n9.pcap \
//	           --static-context static_context.json \
//	           --dialect Keysight_N9 --dry-run
//
//	# live sniffing + send to GoBGP:
//	mup-ribgen --interface eth0 \
//	           --config config.json \
//	           --gobgp-addr 127.0.0.1:50051 --route-type type1
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	// blank import to register all compiled dialect transformers
	_ "github.com/qooqle/mup-ribgen/pkg/dialect"

	"github.com/qooqle/mup-ribgen/pkg/bgp"
	"github.com/qooqle/mup-ribgen/pkg/config"
	"github.com/qooqle/mup-ribgen/pkg/dialect"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/mode1"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
	"github.com/qooqle/mup-ribgen/pkg/pipeline"
	"github.com/qooqle/mup-ribgen/pkg/staticctx"
)

// version is set at build time via -ldflags="-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	// --- flags ---------------------------------------------------------------
	var (
		flagConfig        = flag.String("config", "", "main config file path (req 11.2; default: ./config.json)")
		flagPCAP          = flag.String("pcap", "", "PCAP file to replay (Mode 1, no live capture)")
		flagInterface     = flag.String("interface", "", "network interface to sniff (live Mode 1)")
		flagStaticCtx     = flag.String("static-context", "", "static context JSON config (overrides config file)")
		flagDialect       = flag.String("dialect", "", "PFCP dialect transformer name (overrides config file)")
		flagDryRun        = flag.Bool("dry-run", false, "print BGP routes instead of sending to GoBGP")
		flagGoBGPAddr     = flag.String("gobgp-addr", "", "GoBGP daemon gRPC address host:port (overrides config file)")
		flagRouteType     = flag.String("route-type", "both", "MUP SAFI route type: type1, type2, or both")
		flagLogLevel      = flag.String("log-level", "", "log level: debug, info, warn, error (overrides config file)")
		flagVersion       = flag.Bool("version", false, "print version and exit")
		flagChannelBuffer = flag.Int("channel-buffer", 128, "event channel buffer size")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println("mup-ribgen version", version)
		os.Exit(0)
	}

	// --- config file (req 11.2, 11.3, 1.5) ----------------------------------
	// Track which flags were explicitly set on the command line.
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	cfg, cfgErr := config.Load(*flagConfig)
	if cfgErr != nil && *flagConfig != "" {
		// Non-empty --config that fails to load is fatal (req 9.6).
		fmt.Fprintf(os.Stderr, "mup-ribgen: %v\n", cfgErr)
		os.Exit(1)
	}
	if cfg != nil {
		// Apply config file values for flags that were not explicitly set.
		if !explicit["log-level"] && cfg.LogLevel != "" {
			*flagLogLevel = cfg.LogLevel
		}
		if !explicit["dialect"] && cfg.Dialect != "" {
			*flagDialect = cfg.Dialect
		}
		if !explicit["gobgp-addr"] && cfg.GoBGPAddress != "" {
			*flagGoBGPAddr = cfg.GoBGPAddress
		}
		if !explicit["static-context"] && cfg.StaticContextFile != "" {
			*flagStaticCtx = cfg.StaticContextFile
		}
		if !explicit["interface"] && cfg.PFCPInterface != "" {
			*flagInterface = cfg.PFCPInterface
		}
	}

	// Apply hardcoded defaults for values still unset after config load.
	if *flagLogLevel == "" {
		*flagLogLevel = "info"
	}
	if *flagDialect == "" {
		*flagDialect = "Keysight_N9"
	}
	if *flagGoBGPAddr == "" {
		*flagGoBGPAddr = "127.0.0.1:50051"
	}
	if *flagStaticCtx == "" {
		*flagStaticCtx = "static_context.json"
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

	// --- dialect transformer (req 11.7, 11.8) --------------------------------
	transformer, err := dialect.Lookup(*flagDialect)
	if err != nil {
		slog.Error("dialect not found (req 11.8)", "dialect", *flagDialect,
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
	irMgr.StartPendingDeleteWatcher(ctx)

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

func (d *dryRunSender) print(op, routeType string, rib *ir.BGPRIBInfo) error {
	b, _ := json.Marshal(buildDryRunOutput(op, routeType, rib))
	fmt.Println(string(b))
	return nil
}

func buildDryRunOutput(op, routeType string, rib *ir.BGPRIBInfo) map[string]interface{} {
	out := map[string]interface{}{
		"op":               op,
		"route_type":       routeType,
		"seid":             rib.SEID,
		"route_key":        rib.RouteKey,
		"far_id":           rib.FARID,
		"network_instance": rib.NetworkInstance,
		"rd":               rib.RD,
		"rt":               rib.RT,
		"nexthop":          rib.NexthopAddress,
	}
	switch routeType {
	case "type2":
		out["endpoint"] = rib.EndpointAddress
		out["teid"] = rib.TEID
		if rib.EndpointAddressLength != nil {
			out["endpoint_address_length"] = *rib.EndpointAddressLength
		}
		if rib.MUPExtendedCommunity != nil {
			out["mup_extended_community"] = map[string]interface{}{
				"segment_identifier": hex.EncodeToString(rib.MUPExtendedCommunity.SegmentIdentifier[:]),
			}
		}
	default: // type1
		if rib.UEPrefix != "" {
			out["ue_prefix"] = rib.UEPrefix
		} else {
			out["ue_ip"] = rib.UEIPAddress
		}
		out["endpoint"] = rib.EndpointAddress
		out["teid"] = rib.TEID
		out["qfi"] = rib.QFI
		if rib.SourceAddress != nil && *rib.SourceAddress != "" {
			out["source_address"] = *rib.SourceAddress
		}
	}
	return out
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
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return
	}
	host = h
	port = n
	return
}
