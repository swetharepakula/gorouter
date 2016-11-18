package round_trippers

import (
	"net/http"

	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/lager"
)

type AfterRoundTrip func(rsp *http.Response, endpoint *route.Endpoint, err error)

func NewProxyRoundTripper(typeRoundTripper string, transport http.RoundTripper, endpointIterator route.EndpointIterator,
	logger lager.Logger, afterRoundTrip AfterRoundTrip) http.RoundTripper {
	switch typeRoundTripper {
	case "backend":
		return NewBackendRoundTripper(transport, endpointIterator, afterRoundTrip, logger)
	case "route_service":
		return NewRouteServiceRoundTripper(transport, afterRoundTrip, logger)
		// case "tcp":
		// 	return NewTcpRoundTripper(hijacker, endpointIterator, afterRoundTrip, logger)
	}

	return nil
}
