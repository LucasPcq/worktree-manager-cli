package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ServerParams struct {
	Port     int
	Registry *Registry
}

type Server struct {
	registry *Registry
	listener net.Listener
	http     *http.Server
	port     int
}

func NewServer(params ServerParams) *Server {
	return &Server{registry: params.Registry, port: params.Port}
}

// Start binds the loopback and serves until Close. The bind error is returned
// rather than fatal: a busy port costs the names, never the jobs.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(domain.ProxyLoopbackFmt, s.port))
	if err != nil {
		return err
	}
	s.listener = listener
	s.http = &http.Server{Handler: http.HandlerFunc(s.route)}

	go func() { _ = s.http.Serve(listener) }()
	return nil
}

// Addr is the address the listener actually took, which a port of 0 only knows
// once it is bound.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	route, found := s.registry.Lookup(host)
	if !found {
		s.writeUnknownHost(w, host)
		return
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetXForwarded()
			pr.SetURL(&url.URL{Scheme: domain.ProxyScheme, Host: route.Target})
			// The inbound Host is what the browser scopes cookies to; forwarding
			// it unchanged is the isolation this whole feature exists for.
			pr.Out.Host = pr.In.Host
		},
		ErrorHandler: func(ew http.ResponseWriter, _ *http.Request, _ error) {
			writeSilentTarget(ew, route)
		},
	}
	rp.ServeHTTP(w, r)
}

func (s *Server) writeUnknownHost(w http.ResponseWriter, host string) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, domain.ProxyUnknownHostFmt, host)

	fmt.Fprint(w, domain.ProxyKnownRoutesHead)
	routes := s.registry.List()
	if len(routes) == 0 {
		fmt.Fprint(w, domain.ProxyNoRoutesLine)
		return
	}
	for _, route := range routes {
		fmt.Fprintf(w, domain.ProxyRouteLineFmt, route.Host, route.Target, route.Job, route.Worktree, route.Project)
	}
}

func writeSilentTarget(w http.ResponseWriter, route domain.ProxyRoute) {
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, domain.ProxySilentTargetFmt, route.Job, route.Host, route.Target)
}
