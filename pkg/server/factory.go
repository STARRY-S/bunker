package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/STARRY-S/bunker/pkg/config"
	"github.com/sirupsen/logrus"
)

type FactoryKind int

const (
	APIFactory FactoryKind = iota // Default Factory Kind
	ManifestFactory
	BlobsFactory
)

func (k *FactoryKind) String() string {
	if k == nil {
		return "<nil>"
	}
	switch *k {
	case APIFactory:
		return "API"
	case ManifestFactory:
		return "Manifest"
	case BlobsFactory:
		return "Blobs"
	default:
		return "UNKNOW"
	}
}

const (
	CacheControlHeaderKey = "Cache-Control"
	NoCacheHeader         = "no-store, no-cache, max-age=0, must-revalidate, proxy-revalidate"

	// 604800: 7 days;
	Cache7DaysHeader = "max-age=604800"
	// 864000: 10 days;
	Cache10DaysHeader = "max-age=864000"
)

type factory struct {
	routeType config.RouteType
	name      string
	remoteURL *url.URL
	localURL  *url.URL

	insecureSkipTLSVerify bool

	director       func(r *http.Request)
	modifyResponse func(r *http.Response) error
	errorHandler   func(w http.ResponseWriter, r *http.Request, err error)

	serverErrCh chan error
}

func (f *factory) defaultDirector(r *http.Request) {
	// Change host
	r.URL.Scheme = f.localURL.Scheme
	r.URL.Host = f.localURL.Host

	r.Host = f.remoteURL.Host
	r.Header.Set("Host", f.remoteURL.Host)

	// Dump request debug data
	if logrus.GetLevel() >= logrus.DebugLevel {
		b, err := httputil.DumpRequest(r, false)
		if err != nil {
			logrus.Debugf("failed to dump request: %v", err)
		} else {
			logrus.Debugf("%v Factory MODIFIED REQUEST %q\n%v",
				f.routeType, r.URL, string(b))
		}
	}
}

func (f *factory) defaultModifyResponse(r *http.Response) error {
	if logrus.GetLevel() >= logrus.DebugLevel {
		// Dump response debug data
		b, err := httputil.DumpResponse(r, false)
		if err != nil {
			logrus.Debugf("failed to dump response: %v", err)
		} else {
			logrus.Debugf("%v Factory MODIFIED RESPONSE %q\n%v",
				f.routeType, r.Request.URL, string(b))
		}
	}
	return nil
}

func (f *factory) defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	logrus.Errorf("Error on %v factory handler [%v]: %v",
		f.routeType, r.URL.Path, err)
	b, _ := httputil.DumpRequest(r, true)
	logrus.Errorf("%v Factory failed request %q\n%v\n=========================\n",
		f.routeType, r.URL.String(), string(b))

	w.WriteHeader(http.StatusBadGateway)
	w.Write(fmt.Appendf(nil, "%v", err))

	if strings.Contains(err.Error(), http.StatusText(http.StatusBadGateway)) {
		if f.serverErrCh == nil {
			return
		}
		f.serverErrCh <- fmt.Errorf("server failed on proxy error: %w", err)
	}
}

// Proxy generates the ReverseProxy server
func (f *factory) Proxy() *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(f.remoteURL)
	p.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: f.insecureSkipTLSVerify,
		},
	}
	p.Director = f.director
	p.ModifyResponse = f.modifyResponse
	p.ErrorHandler = f.errorHandler
	return p
}
