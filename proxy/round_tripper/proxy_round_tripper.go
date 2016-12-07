package round_tripper

import (
	"io/ioutil"
	"net"
	"net/http"

	"github.com/uber-go/zap"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/route"
)

type AfterRoundTrip func(rsp *http.Response, endpoint *route.Endpoint, err error)

func NewProxyRoundTripper(backend bool, transport http.RoundTripper, endpointIterator route.EndpointIterator,
	logger zap.Logger, afterRoundTrip AfterRoundTrip) http.RoundTripper {
	if backend {
		return &BackendRoundTripper{
			transport: transport,
			iter:      endpointIterator,
			logger:    logger,
			after:     afterRoundTrip,
		}
	} else {
		return &RouteServiceRoundTripper{
			transport: transport,
			logger:    logger,
			after:     afterRoundTrip,
		}
	}
}

type BackendRoundTripper struct {
	iter      route.EndpointIterator
	transport http.RoundTripper
	logger    zap.Logger
	after     AfterRoundTrip
}

func (rt *BackendRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	var err error
	var res *http.Response
	var endpoint *route.Endpoint

	if request.Body != nil {
		closer := request.Body
		request.Body = ioutil.NopCloser(request.Body)
		defer func() {
			closer.Close()
		}()
	}

	for retry := 0; retry < handler.MaxRetries; retry++ {
		endpoint, err = rt.selectEndpoint(request)
		if err != nil {
			break
		}

		rt.setupRequest(request, endpoint)

		// increment connection stats
		rt.iter.PreRequest(endpoint)

		res, err = rt.transport.RoundTrip(request)

		// decrement connection stats
		rt.iter.PostRequest(endpoint)

		if err == nil || !retryableError(err) {
			break
		}

		rt.reportError(err)
	}

	if err != nil {
		rt.logger.Error("endpoint-failed", zap.Error(err))
	}

	if rt.after != nil {
		rt.after(res, endpoint, err)
	}

	return res, err
}

func (rt *BackendRoundTripper) selectEndpoint(request *http.Request) (*route.Endpoint, error) {
	endpoint := rt.iter.Next()
	if endpoint == nil {
		return nil, handler.NoEndpointsAvailable
	}

	rt.logger = rt.logger.With(zap.Object("route-endpoint", endpoint.ToLogData()))
	return endpoint, nil
}

func (rt *BackendRoundTripper) setupRequest(request *http.Request, endpoint *route.Endpoint) {
	rt.logger.Debug("backend")
	request.URL.Host = endpoint.CanonicalAddr()
	request.Header.Set("X-CF-ApplicationID", endpoint.ApplicationId)
	handler.SetRequestXCfInstanceId(request, endpoint)
}

func (rt *BackendRoundTripper) reportError(err error) {
	rt.iter.EndpointFailed()
	rt.logger.Error("backend-endpoint-failed", zap.Error(err))
}

type RouteServiceRoundTripper struct {
	transport http.RoundTripper
	after     AfterRoundTrip
	logger    zap.Logger
}

func (rt *RouteServiceRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	var err error
	var res *http.Response

	for retry := 0; retry < handler.MaxRetries; retry++ {
		res, err = rt.transport.RoundTrip(request)
		if err == nil || !retryableError(err) {
			break
		}

		rt.reportError(err)
	}

	if rt.after != nil {
		endpoint := newRouteServiceEndpoint()
		rt.after(res, endpoint, err)
	}

	return res, err
}

func (rs *RouteServiceRoundTripper) reportError(err error) {
	rs.logger.Error("route-service-failed", zap.Error(err))
}

func retryableError(err error) bool {
	ne, netErr := err.(*net.OpError)
	if netErr && ne.Op == "dial" {
		return true
	}

	return false
}

func newRouteServiceEndpoint() *route.Endpoint {
	return &route.Endpoint{
		Tags: map[string]string{},
	}
}
