package nlmt

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	flag "github.com/ogier/pflag"
)

func serverUsage() {
	setBufio()
	printf("Options:")
	printf("--------")
	printf("")
	printf("-b addresses   bind addresses (default \"%s\"), comma separated list of:", strings.Join(DefaultBindAddrs, ","))
	printf("               :port (unspecified address with port, use with care)")
	printf("               host (host with default port %s, see Host formats below)", DefaultPort)
	printf("               host:port (host with specified port, see Host formats below)")
	printf("               %%iface (all addresses on interface iface with default port %s)", DefaultPort)
	printf("               %%iface:port (all addresses on interface iface with port)")
	printf("               note: iface strings may contain * to match multiple interfaces")
	printf("-o file        write JSON output to file (use 'd' for default filename)")
	printf("               if file has no extension, .json.gz is added, output is gzipped")
	printf("               if extension is .json.gz, output is gzipped")
	printf("               if extension is .gz, it's changed to .json.gz, output is gzipped")
	printf("               if extension is .json, output is not gzipped")
	printf("               output to stdout is not gzipped, pipe to gzip if needed")
	printf("--outdir=dir   output files directory folder (default %s)", DefaultOutputDir)
	printf("               only applies if default file name is used.")
	printf("-q             quiet, suppress per-packet output")
	printf("-Q             really quiet, suppress all output except errors to stderr")
	printf("-d duration    max test duration, or 0 for no maximum")
	printf("               (default %s, see Duration units below)", DefaultMaxDuration)
	printf("-i interval    min send interval, or 0 for no minimum")
	printf("               (default %s, see Duration units below)", DefaultMinInterval)
	printf("-l length      max packet length (default %d), or 0 for no maximum", DefaultMaxLength)
	printf("               numbers too small will cause test packets to be dropped")
	printf("-n net print   print logs to a server e.g. \"0.0.0.0:50009\" (default disabled)")
	printf("--hmac=key     add HMAC with key (0x for hex) to all packets, provides:")
	printf("               dropping of all packets without a correct HMAC")
	printf("               protection for server against unauthorized discovery and use")
	if syslogSupport {
		printf("--syslog=uri   log events to syslog (default don't use syslog)")
		printf("               URI format: scheme://host:port/tag, examples:")
		printf("               local: log to local syslog, default tag irtt")
		printf("               local:/irttsrv: log to local syslog, tag irttsrv")
		printf("               udp://logsrv:514/irttsrv: UDP to logsrv:514, tag irttsrv")
		printf("               tcp://logsrv:8514/: TCP to logsrv:8514, default tag irtt")
	}
	printf("--timeout=dur  timeout for closing connections if no requests received")
	printf("               0 means no timeout (not recommended on public servers)")
	printf("               max client interval will be restricted to timeout/%d", maxIntervalTimeoutFactor)
	printf("               (default %s, see Duration units below)", DefaultServerTimeout)
	printf("--pburst=#     packet burst allowed before enforcing minimum interval")
	printf("               (default %d)", DefaultPacketBurst)
	printf("--fill=fill    payload fill if not requested (default %s)", DefaultServerFiller.String())
	printf("               none: echo client payload (insecure on public servers)")
	for _, ffac := range FillerFactories {
		printf("               %s", ffac.Usage)
	}
	printf("--allow-fills= comma separated patterns of fill requests to allow (default %s)", strings.Join(DefaultAllowFills, ","))
	printf("  fills        see options for --fill")
	printf("               allowing non-random fills insecure on public servers")
	printf("               use --allow-fills=\"\" to disallow all fill requests")
	printf("               note: patterns may contain * for matching")
	printf("--tstamp=modes timestamp modes to allow (default %s)", DefaultAllowStamp)
	printf("               none: don't allow timestamps")
	printf("               single: allow a single timestamp (send, receive or midpoint)")
	printf("               dual: allow dual timestamps")
	printf("--no-dscp      don't allow setting dscp (default %t)", !DefaultAllowDSCP)
	printf("-4             IPv4 only")
	printf("-6             IPv6 only")
	printf("--set-src-ip   set source IP address on all outgoing packets from listeners")
	printf("               on unspecified IP addresses (use for more reliable reply")
	printf("               routing, but increases per-packet heap allocations)")
	printf("--ecn          Ship ECN bits to be logged by the client.  Forces --set-src-ip, disables UDP replies")
	printf("--thread       lock request handling goroutines to OS threads")
	printf("-h             show help")
	printf("-v             show version")
	printf("")
	hostUsage()
	printf("")
	durationUsage()
}

// runServerCLI runs the server command line interface.
func runServerCLI(args []string) {
	// server flags
	fs := flag.NewFlagSet("server", 0)
	fs.Usage = func() {
		usageAndExit(serverUsage, exitCodeBadCommandLine)
	}
	var baddrsStr = fs.StringP("b", "b", strings.Join(DefaultBindAddrs, ","), "bind addresses")
	var maxDuration = fs.DurationP("d", "d", DefaultMaxDuration, "max duration")
	var minInterval = fs.DurationP("i", "i", DefaultMinInterval, "min interval")
	var maxLength = fs.IntP("l", "l", DefaultMaxLength, "max length")
	var netprintAddr = fs.StringP("n", "n", "", "netprint address")
	var allowTimestampStr = fs.String("tstamp", DefaultAllowStamp.String(), "allow timestamp")
	var hmacStr = fs.String("hmac", defaultHMACKey, "HMAC key")
	var syslogStr *string
	if syslogSupport {
		syslogStr = fs.String("syslog", "", "syslog uri")
	}
	var timeout = fs.Duration("timeout", DefaultServerTimeout, "timeout")
	var packetBurst = fs.Int("pburst", DefaultPacketBurst, "packet burst")
	var fillStr = fs.String("fill", DefaultServerFiller.String(), "fill")
	var allowFillsStr = fs.String("allow-fills", strings.Join(DefaultAllowFills, ","), "sfill")
	var ipv4 = fs.BoolP("4", "4", false, "IPv4 only")
	var ipv6 = fs.BoolP("6", "6", false, "IPv6 only")
	var ttl = fs.Int("ttl", DefaultTTL, "IP time to live")
	var noDSCP = fs.Bool("no-dscp", !DefaultAllowDSCP, "no DSCP")
	var setSrcIP = fs.Bool("set-src-ip", DefaultSetSrcIP, "set source IP")
	var ecn = fs.Bool("ecn", DefaultSetECN, "enable ECN capture - disables UDP replies from server")
	var lockOSThread = fs.Bool("thread", DefaultThreadLock, "thread")
	var version = fs.BoolP("version", "v", false, "version")
	var outputStr = fs.StringP("o", "o", "", "output file")
	var outputDirStr = fs.String("outdir", DefaultOutputDir, "output directory")
	var quiet = fs.BoolP("q", "q", defaultQuiet, "quiet")
	var reallyQuiet = fs.BoolP("Q", "Q", defaultReallyQuiet, "really quiet")
	fs.Parse(args)

	// start profiling, if enabled in build
	if profileEnabled {
		defer startProfile("./server.pprof").Stop()
	}

	// version
	if *version {
		runVersion(args)
		os.Exit(0)
	}

	// determine IP version
	ipVer := IPVersionFromBooleans(*ipv4, *ipv6, DualStack)

	// parse allow stamp string
	allowStamp, err := ParseAllowStamp(*allowTimestampStr)
	exitOnError(err, exitCodeBadCommandLine)

	// parse fill
	filler, err := NewFiller(*fillStr)
	exitOnError(err, exitCodeBadCommandLine)

	// parse HMAC key
	var hmacKey []byte
	if *hmacStr != "" {
		hmacKey, err = decodeHexOrNot(*hmacStr)
		exitOnError(err, exitCodeBadCommandLine)
	}

	// create event handler with console handler as default
	handler := &MultiHandler{[]Handler{&consoleHandler{}}}

	// add syslog event handler
	if syslogStr != nil && *syslogStr != "" {
		sh, err := newSyslogHandler(*syslogStr)
		exitOnError(err, exitCodeRuntimeError)
		handler.AddHandler(sh)
	}

	// create server config
	cfg := NewServerConfig()
	cfg.Addrs = strings.Split(*baddrsStr, ",")
	cfg.MaxDuration = *maxDuration
	cfg.MinInterval = *minInterval
	cfg.AllowStamp = allowStamp
	cfg.HMACKey = hmacKey
	cfg.Timeout = *timeout
	cfg.PacketBurst = *packetBurst
	cfg.MaxLength = *maxLength
	cfg.Filler = filler
	cfg.AllowFills = strings.Split(*allowFillsStr, ",")
	cfg.AllowDSCP = !*noDSCP
	cfg.TTL = *ttl
	cfg.Handler = handler
	cfg.IPVersion = ipVer
	cfg.SetSrcIP = *setSrcIP || *ecn
	cfg.ThreadLock = *lockOSThread
	cfg.Quiet = *quiet
	cfg.ReallyQuiet = *reallyQuiet
	if *outputStr == "" {
		cfg.OutputJSON = false
	} else if *outputStr == "d" {
		cfg.OutputDir = *outputDirStr
		cfg.OutputJSON = true
	} else {
		cfg.OutputJSON = true
		cfg.OutputJSONAddr = *outputStr
	}

	// connect to the print server if set
	if *netprintAddr != "" {
		// Create a TCP connection to the server
		cfg.netprintp, err = NewNetPrint(*netprintAddr)
		if err != nil {
			fmt.Println("Error connecting to the netprint server: ", err)
			exitOnError(err, exitCodeBadCommandLine)
		}
		defer cfg.netprintp.CloseConnection()
	}

	// create server
	s := NewServer(cfg)

	// install signal handler to stop server
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		printf("%s", sig)
		s.Shutdown()

		sig = <-sigs
		os.Exit(exitCodeDoubleSignal)
	}()

	err = s.ListenAndServe()
	exitOnError(err, exitCodeRuntimeError)
}
