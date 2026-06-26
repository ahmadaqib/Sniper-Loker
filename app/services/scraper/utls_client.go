package scraper

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

func NewUTLSHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	transport.ForceAttemptHTTP2 = false

	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var dialer net.Dialer
		rawConn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			_ = rawConn.Close()
			return nil, err
		}

		tlsConn := utls.UClient(rawConn, &utls.Config{ServerName: host, InsecureSkipVerify: true}, utls.HelloCustom)
		spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
		if err != nil {
			_ = rawConn.Close()
			return nil, err
		}

		// Remove h2 from ALPN to force HTTP/1.1
		for _, ext := range spec.Extensions {
			if alpn, ok := ext.(*utls.ALPNExtension); ok {
				alpn.AlpnProtocols = []string{"http/1.1"}
			}
		}
		if err := tlsConn.ApplyPreset(&spec); err != nil {
			_ = rawConn.Close()
			return nil, err
		}

		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}

		return tlsConn, nil
	}

	return &http.Client{Timeout: timeout, Transport: transport}
}
