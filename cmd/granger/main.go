package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"granger/internal/config"
	"granger/internal/engine"
	"granger/internal/health"
	"granger/pkg/runner"
)

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	r := runner.New()
	e := engine.New(r)
	h := health.New(r)
	switch cmd {
	case "help", "-h", "--help":
		fmt.Println("usage: granger [apply|health|runtime|drivers|restart-output NAME|restart-upstream NAME|test-domain DOMAIN]")
		fmt.Println()
		fmt.Println("commands:")
		fmt.Println("  apply                 apply declarative routing config")
		fmt.Println("  health                run health checks")
		fmt.Println("  runtime               show output/upstream driver runtime states")
		fmt.Println("  drivers               list registered upstream/output drivers")
		fmt.Println("  restart-output NAME   restart output by config name")
		fmt.Println("  restart-upstream NAME restart upstream by config name")
		fmt.Println("  test-domain DOMAIN    resolve/test domain against Granger DNS")
	case "apply":
		cfg := mustConfig()
		for _, res := range e.ApplyConfig(cfg).Results {
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
		fmt.Fprintln(os.Stderr, "usage: granger [apply|health|runtime|drivers|restart-output NAME|restart-upstream NAME|test-domain DOMAIN]")
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
