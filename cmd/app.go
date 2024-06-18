package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-kind/pkg/config"
	"sigs.k8s.io/cloud-provider-kind/pkg/controller"
)

var (
	flagV         int
	enableLogDump bool
	logDumpDir    string
)

func init() {
	flag.IntVar(&flagV, "v", 2, "Verbosity level")
	flag.BoolVar(&enableLogDump, "enable-log-dumping", false, "store logs to a temporal directory or to the directory specified using the logs-dir flag")
	flag.StringVar(&logDumpDir, "logs-dir", "", "store logs to the specified directory")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cloud-provider-kind [options]\n\n")
		flag.PrintDefaults()
	}
}

func Main() {
	// Parse command line flags and arguments
	flag.Parse()
	flag.VisitAll(func(flag *flag.Flag) {
		klog.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})

	// trap Ctrl+C and call cancel on the context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Enable signal handler
	signalCh := make(chan os.Signal, 2)
	defer func() {
		close(signalCh)
		cancel()
	}()

	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-signalCh:
			klog.Infof("Exiting: received signal")
			cancel()
		case <-ctx.Done():
			// cleanup
		}
	}()

	_, _ = logs.GlogSetter(strconv.Itoa(flagV))

	// initialize log directory
	if enableLogDump {
		if logDumpDir == "" {
			dir, err := os.MkdirTemp(os.TempDir(), "kind-provider-")
			if err != nil {
				klog.Fatal(err)
			}
			logDumpDir = dir
		}

		if _, err := os.Stat(logDumpDir); os.IsNotExist(err) {
			if err := os.MkdirAll(logDumpDir, 0755); err != nil {
				klog.Fatalf("directory %s does not exist: %v", logDumpDir, err)
			}
		}
		config.DefaultConfig.EnableLogDump = true
		config.DefaultConfig.LogDir = logDumpDir
		klog.Infof("**** Dumping load balancers logs to: %s", logDumpDir)
	}

	controller.New().Run(ctx)
}
