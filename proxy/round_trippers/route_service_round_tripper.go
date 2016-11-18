package round_trippers

import (
	"net"
	"net/http"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/lager"
)

type routeServiceRoundTripper struct {
	transport http.RoundTripper
	after     AfterRoundTrip
	logger    lager.Logger
}

func NewRouteServiceRoundTripper(
	transport http.RoundTripper,
	after AfterRoundTrip,
	logger lager.Logger,
) http.RoundTripper {
	return &routeServiceRoundTripper{
		transport: transport,
		after:     after,
		logger:    logger,
	}
}

func (rt *routeServiceRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
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

func (rs *routeServiceRoundTripper) reportError(err error) {
	rs.logger.Error("route-service-failed", err)
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
