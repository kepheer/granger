package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"granger/internal/config"
	"granger/internal/engine"
	"granger/internal/health"
	"granger/internal/webgui"
	"granger/pkg/runner"
)

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	dryRun := false
	if cmd == "apply" && len(os.Args) > 2 && os.Args[2] == "--dry-run" {
		dryRun = true
	}
	r := runner.New()
	if dryRun {
		r = runner.NewDryRun()
	}
	e := engine.New(r)
	h := health.New(r)
	switch cmd {
	case "help", "-h", "--help":
		fmt.Println("usage: granger [apply|health|runtime|drivers|serve-gui [ADDR] [DIR]|restart-output NAME|restart-upstream NAME|test-domain DOMAIN]")
		fmt.Println()
		fmt.Println("commands:")
		fmt.Println("  apply [--dry-run]     apply declarative routing config")
		fmt.Println("  health                run health checks")
		fmt.Println("  runtime               show output/upstream driver runtime states")
		fmt.Println("  drivers               list registered upstream/output drivers")
		fmt.Println("  serve-gui [ADDR] [DIR] serve static GUI, default 10.19.84.51:1984 dist/gui")
		fmt.Println("  restart-output NAME   restart output by config name")
		fmt.Println("  restart-upstream NAME restart upstream by config name")
		fmt.Println("  test-domain DOMAIN    resolve/test domain against Granger DNS")
	case "apply":
		cfg := mustConfig()
		plan := e.ApplyConfig(cfg)
		fmt.Printf("firewall backend: %s\n", plan.Firewall)
		if plan.DryRun {
			fmt.Println("mode: dry-run")
		}
		fmt.Println()
		for _, res := range plan.Results {
			fmt.Printf("[%v] %s\n$ %s\n%s\n\n", res.OK, res.Title, res.Command, res.Output)
		}
	case "health":
		cfg := mustConfig()
		st := h.Check(cfg)
		fmt.Println("overall:", st.Overall)
		for _, ch := range st.Checks {
			fmt.Printf("%s: %s - %s\n", ch.Name, ch.State, ch.Summary)
		}
	case "drivers":
		up, out := e.SupportedDrivers()
		fmt.Println("upstreams:", strings.Join(up, ", "))
		fmt.Println("outputs:", strings.Join(out, ", "))
	case "runtime":
		cfg := mustConfig()
		for _, st := range e.Runtime(cfg) {
			fmt.Printf("%s (%s): %s - %s\n", st.Name, st.Type, st.State, st.Summary)
		}
	case "serve-gui":
		srv := webgui.ConfigFromEnv()
		if len(os.Args) > 2 {
			srv.Listen = os.Args[2]
		}
		if len(os.Args) > 3 {
			srv.Dir = os.Args[3]
		}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	case "restart-output":
		if len(os.Args) < 3 {
			log.Fatal("usage: granger restart-output NAME")
		}
		cfg := mustConfig()
		for _, res := range e.RestartOutput(os.Args[2], cfg) {
			fmt.Printf("[%v] %s\n$ %s\n%s\n\n", res.OK, res.Title, res.Command, res.Output)
		}
	case "restart-upstream":
		if len(os.Args) < 3 {
			log.Fatal("usage: granger restart-upstream NAME")
		}
		cfg := mustConfig()
		for _, res := range e.RestartUpstream(os.Args[2], cfg) {
			fmt.Printf("[%v] %s\n$ %s\n%s\n\n", res.OK, res.Title, res.Command, res.Output)
		}
	case "test-domain":
		if len(os.Args) < 3 {
			log.Fatal("usage: granger test-domain example.org")
		}
		cfg := mustConfig()
		for _, res := range e.TestDomain(os.Args[2], cfg) {
			fmt.Printf("[%v] %s\n$ %s\n%s\n\n", res.OK, res.Title, res.Command, res.Output)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: granger [apply|health|runtime|drivers|serve-gui [ADDR] [DIR]|restart-output NAME|restart-upstream NAME|test-domain DOMAIN]")
		os.Exit(2)
	}
}

func mustConfig() config.Config {
	cfg, err := config.Load(config.Path)
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}
