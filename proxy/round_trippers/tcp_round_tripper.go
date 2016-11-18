package round_trippers

import (
	"io"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/lager"
)

type tcpRoundTripper struct {
	iter     route.EndpointIterator
	hijacker http.Hijacker
	logger   lager.Logger
	after    AfterRoundTrip
}

func NewTcpRoundTripper(
	hijacker http.Hijacker,
	iter route.EndpointIterator,
	after AfterRoundTrip,
	logger lager.Logger,
) http.RoundTripper {
	return &tcpRoundTripper{
		hijacker: hijacker,
		iter:     iter,
		after:    after,
		logger:   logger,
	}
}

func (rt *tcpRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	var err error
	var connection net.Conn
	resp := &http.Response{}

	rt.logger.Info("handling-tcp-request", lager.Data{"Upgrade": "tcp"})

	client, _, err := rt.hijacker.Hijack()
	if err != nil {
		rt.logger.Error("tcp-request-failed", err)
		if rt.after != nil {
			rt.after(resp, nil, err)
		}
		return resp, err
	}

	defer func() {
		client.Close()
		if connection != nil {
			connection.Close()
		}
	}()

	retries := 0
	for {
		endpoint := rt.iter.Next()
		if endpoint == nil {
			if rt.after != nil {
				rt.after(resp, endpoint, handler.NoEndpointsAvailable)
			}
			return nil, handler.NoEndpointsAvailable
		}

		addr := endpoint.CanonicalAddr()

		connection, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			break
		}

		rt.logger.Error("tcp-connection-failed", err, lager.Data{"route-endpoint": addr})
		retries++
		if retries == handler.MaxRetries {
			return nil, err
		}
	}

	resp.StatusCode = http.StatusSwitchingProtocols

	forwardIO(client, connection)
	return resp, nil
}

func forwardIO(a, b net.Conn) {
	done := make(chan bool, 2)

	copy := func(dst io.Writer, src io.Reader) {
		// don't care about errors here
		io.Copy(dst, src)
		done <- true
	}

	go copy(a, b)
	go copy(b, a)

	<-done
}
