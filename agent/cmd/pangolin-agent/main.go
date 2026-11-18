package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/os-pangolin/os-pangolin/agent/internal/agent"
	"github.com/os-pangolin/os-pangolin/agent/internal/config"
	"github.com/os-pangolin/os-pangolin/agent/internal/logging"
	"github.com/os-pangolin/os-pangolin/agent/internal/state"
	"github.com/os-pangolin/os-pangolin/agent/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "dns-records":
		err = cmdDNSRecords(os.Args[2:])
	case "test":
		err = cmdTest(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("os-pangolin-agent %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	case "help", "--help", "-h":
		usage()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "os-pangolin-agent: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `os-pangolin-agent - native Pangolin client/site connector for OPNsense

Usage:
  pangolin-agent run --config <path> [--state-dir DIR] [--log-level LEVEL]
  pangolin-agent status --mode site|client [--state-dir DIR] [--json]
  pangolin-agent validate --config <path>
  pangolin-agent dns-records [--state-dir DIR] [--json]
  pangolin-agent test (--endpoint URL | --config PATH)
  pangolin-agent version
`)
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to JSON configuration")
	stateDir := fs.String("state-dir", "", "override state directory")
	logLevel := fs.String("log-level", "", "override log level")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *stateDir != "" {
		cfg.StateDir = *stateDir
		if cfg.DNS != nil {
			cfg.DNS.OutputFile = filepath.Join(cfg.StateDir, config.DefaultDNSOutput)
		}
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	logging.Init(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return agent.Run(ctx, cfg)
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	mode := fs.String("mode", "", "site or client")
	stateDir := fs.String("state-dir", config.DefaultStateDir, "state directory")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *mode != config.ModeSite && *mode != config.ModeClient {
		return fmt.Errorf("--mode must be site or client")
	}
	path := filepath.Join(*stateDir, *mode+"-status.json")
	st, err := state.ReadStatus(path)
	if err != nil {
		st = state.Status{
			Version: version.Version,
			Mode:    *mode,
			LastError: "agent is not running (status file missing)",
		}
	}
	if *asJSON {
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("mode:        %s\n", st.Mode)
	fmt.Printf("pid:         %d\n", st.PID)
	fmt.Printf("connected:   %v\n", st.Connected)
	fmt.Printf("registered:  %v\n", st.Registered)
	if st.TunnelIP != "" {
		fmt.Printf("tunnel IP:   %s\n", st.TunnelIP)
	}
	if st.Interface != "" {
		fmt.Printf("interface:   %s\n", st.Interface)
	}
	if st.LastError != "" {
		fmt.Printf("last error:  %s\n", st.LastError)
	}
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to JSON configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	fmt.Printf("configuration OK: mode=%s stateDir=%s\n", cfg.Mode, cfg.StateDir)
	return nil
}

func cmdDNSRecords(args []string) error {
	fs := flag.NewFlagSet("dns-records", flag.ContinueOnError)
	stateDir := fs.String("state-dir", config.DefaultStateDir, "state directory")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := filepath.Join(*stateDir, config.DefaultDNSOutput)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if *asJSON {
				fmt.Println("[]")
			}
			return nil
		}
		return err
	}
	if *asJSON {
		fmt.Println(string(data))
		return nil
	}
	var records []state.DNSEntry
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}
	for _, r := range records {
		fmt.Printf("%s %s %s\n", r.Domain, r.Type, r.IP)
	}
	return nil
}

func cmdTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	endpoint := fs.String("endpoint", "", "endpoint URL")
	configPath := fs.String("config", "", "read endpoint from configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := *endpoint
	if target == "" && *configPath != "" {
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		switch cfg.Mode {
		case config.ModeSite:
			if cfg.Site != nil {
				target = cfg.Site.Endpoint
			}
		case config.ModeClient:
			if cfg.Client != nil {
				target = cfg.Client.Endpoint
			}
		}
	}
	if target == "" {
		return fmt.Errorf("--endpoint or --config is required")
	}
	return agent.TestEndpoint(context.Background(), target)
}
