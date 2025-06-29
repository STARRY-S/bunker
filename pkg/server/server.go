package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/STARRY-S/bunker/pkg/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
)

type bunkerServer struct {
	addr string // proxy server bind address
	port int    // proxy server bind port

	cert string
	key  string

	insecureSkipTLSVerify bool

	// Custom Route proxy
	customProxyMap map[string]*httputil.ReverseProxy
	// Custom plaintext proxy map, can be cached by CDN in a short period
	plaintextProxySet map[config.Route]bool // map[route]true
	// Custom static file proxy map, can be cached by CDN in a short period
	staticFileProxySet map[config.Route]bool // map[route]true

	server *http.Server   // HTTP2 server
	mux    *http.ServeMux // HTTP request multiplexer
	errCh  chan error
}

func NewBunkerServer(
	ctx context.Context, c *config.Config,
) (*bunkerServer, error) {
	s := &bunkerServer{
		addr: c.BindAddr,
		port: c.Port,

		cert: c.CertFile,
		key:  c.KeyFile,

		insecureSkipTLSVerify: c.InsecureSkipTLSVerify,

		customProxyMap:     make(map[string]*httputil.ReverseProxy),
		plaintextProxySet:  make(map[config.Route]bool),
		staticFileProxySet: make(map[config.Route]bool),

		errCh: make(chan error),
	}
	for _, r := range c.Routes {
		switch r.RouteType {
		case config.TypeDefault, "":
			if err := s.registerCustomFactory(r); err != nil {
				return nil, err
			}
		case config.TypePlainText:
			s.registerPlainText(r)
		case config.TypeStaticFile:
			s.registerStaticFile(r)
		default:
			logrus.Warnf("Ignore custom route %q: unknow type %v",
				r.Name, r.RouteType)
		}
	}

	return s, nil
}

func (s *bunkerServer) registerPlainText(r config.Route) {
	s.plaintextProxySet[r] = true
}

func (s *bunkerServer) registerStaticFile(r config.Route) {
	s.staticFileProxySet[r] = true
}

func (s *bunkerServer) registerCustomFactory(r config.Route) error {
	f := &factory{
		routeType: r.RouteType,
		name:      r.Name,
		localURL:  nil,
		remoteURL: nil,

		insecureSkipTLSVerify: s.insecureSkipTLSVerify,
		serverErrCh:           s.errCh,
	}
	f.errorHandler = f.defaultErrorHandler
	f.modifyResponse = f.defaultModifyResponse
	f.director = f.defaultDirector

	localURL, err := url.Parse(r.LocalURL)
	if err != nil {
		return fmt.Errorf("failed to parse route localURL %q: %w", r.LocalURL, err)
	}
	remoteURL, err := url.Parse(r.RemoteURL)
	if err != nil {
		return fmt.Errorf("failed to parse route remoteURL %q: %w", r.RemoteURL, err)
	}
	f.localURL = localURL
	f.remoteURL = remoteURL

	s.customProxyMap[remoteURL.Host] = f.Proxy()
	logrus.Debugf("Registered custom route %q proxy", remoteURL.String())

	return nil
}

func (s *bunkerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	logrus.Debugf("Proxy path [%v]", path)
	for h, p := range s.customProxyMap {
		if r.Host != h {
			continue
		}
		p.ServeHTTP(w, r)
		return
	}

	for r := range s.plaintextProxySet {
		if !matchCustomRoute(&r, path) {
			continue
		}

		if r.PlainText.Status != 0 {
			w.WriteHeader(r.PlainText.Status)
		}
		w.Write([]byte(r.PlainText.Content))
		logrus.Debugf("response plaintext path [%v] status [%v] content [%v]",
			path, r.PlainText.Status, r.PlainText.Content)
		return
	}

	for r := range s.staticFileProxySet {
		if !matchCustomRoute(&r, path) {
			continue
		}
		if r.StaticFile == "" {
			continue
		}

		f, err := os.Open(r.StaticFile)
		if err != nil {
			logrus.Warnf("failed to open file %q: %v", r.StaticFile, err)
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
		defer f.Close()

		if _, err := io.Copy(w, f); err != nil {
			logrus.Errorf("Failed to response file content: %q: %v",
				r.StaticFile, err)
			return
		}
		logrus.Debugf("response file [%v] prefix [%v]",
			r.StaticFile, path)
		return
	}

	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte("403 forbidden"))
}

func (s *bunkerServer) initServer() error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/", s.ServeHTTP)
	addr := fmt.Sprintf("%v:%v", s.addr, s.port)
	scheme := "http"
	if s.cert != "" && s.key != "" {
		scheme = "https"
	}
	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: time.Second * 10,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: s.insecureSkipTLSVerify,
		},
	}
	if err := http2.ConfigureServer(s.server, &http2.Server{}); err != nil {
		return fmt.Errorf("failed to configure http2 server: %v", err)
	}
	logrus.Infof("server listen on %v://%v", scheme, addr)
	return nil
}

func (s *bunkerServer) waitServerShutDown(ctx context.Context) error {
	select {
	case err := <-s.errCh:
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		s.server.Shutdown(timeoutCtx)
		cancel()
		return err
	case <-ctx.Done():
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		s.server.Shutdown(timeoutCtx)
		cancel()
		logrus.Warnf("%v", ctx.Err())
	}
	return nil
}

func (s *bunkerServer) Serve(ctx context.Context) error {
	if err := s.initServer(); err != nil {
		return err
	}
	go func() {
		var err error
		if s.cert == "" {
			err = s.server.ListenAndServe()
		} else {
			err = s.server.ListenAndServeTLS(s.cert, s.key)
		}

		if err != nil {
			s.errCh <- fmt.Errorf("failed to start server: %w", err)
		}
	}()
	return s.waitServerShutDown(ctx)
}

func matchCustomRoute(r *config.Route, path string) bool {
	switch {
	case r.Path != "":
		return r.Path == path
	case r.Prefix != "":
		return strings.HasPrefix(path, r.Prefix)
	}
	return false
}
