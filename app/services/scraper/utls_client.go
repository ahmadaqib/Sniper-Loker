package scraper

import (
	"context"
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
	transport.TLSClientConfig = nil
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

		tlsConn := utls.UClient(rawConn, &utls.Config{ServerName: host}, utls.HelloChrome_Auto)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}

		return tlsConn, nil
	}

	return &http.Client{Timeout: timeout, Transport: transport}
}
