// package metrics - prometheus metrics for titan
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server - serves prometheus metrics on /metrics endpoint
type Server struct {
	addr string
	srv  *http.Server
}

// NewServer - create a new metrics server, usually on :9090
func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &Server{
		addr: addr,
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

// Start - starts the metrics server (non-blocking)
func (s *Server) Start() error {
	go func() {
		_ = s.srv.ListenAndServe()
	}()
	return nil
}

// Stop - stops the metrics server
func (s *Server) Stop() error {
	return s.srv.Close()
}
