package webgui

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"granger/internal/config"
)

type vpnGUI struct {
	handler http.Handler
	servers map[string]*http.Server
}

func (v *vpnGUI) reconcile(cfg config.Config, primary string) {
	wanted := map[string]bool{}
	if cfg.Server.GUIVPNPort > 0 {
		addresses := append([]string{"127.0.0.1"}, cfg.GUIVPNAddresses()...)
		for _, address := range addresses {
			endpoint := net.JoinHostPort(address, strconv.Itoa(cfg.Server.GUIVPNPort))
			if endpoint != primary {
				wanted[endpoint] = true
			}
		}
	}
	for address, server := range v.servers {
		if wanted[address] == false {
			server.Close()
			delete(v.servers, address)
		}
	}
	for address := range wanted {
		if _, exists := v.servers[address]; exists {
			continue
		}
		listener, err := net.Listen("tcp4", address)
		if err != nil {
			log.Printf("VPN GUI listener %s pending: %v", address, err)
			continue
		}
		server := &http.Server{Handler: v.handler, ReadHeaderTimeout: 5 * time.Second}
		v.servers[address] = server
		go func() {
			if err := server.Serve(listener); err != nil && errors.Is(err, http.ErrServerClosed) == false {
				log.Printf("VPN GUI listener %s failed: %v", listener.Addr(), err)
			}
		}()
		log.Printf("serving VPN GUI on http://%s", address)
	}
}

func watchVPNGUI(ctx context.Context, primary string, handler http.Handler) {
	v := vpnGUI{handler: handler, servers: map[string]*http.Server{}}
	defer func() {
		for _, server := range v.servers {
			server.Close()
		}
	}()
	reconcile := func() {
		cfg, err := loadConfig()
		if err == nil {
			v.reconcile(cfg, primary)
		}
	}
	reconcile()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}
