package round_trippers

import (
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/lager"
)

type backendRoundTripper struct {
	iter      route.EndpointIterator
	transport http.RoundTripper
	logger    lager.Logger
	after     AfterRoundTrip
}

func NewBackendRoundTripper(
	transport http.RoundTripper,
	iter route.EndpointIterator,
	after AfterRoundTrip,
	logger lager.Logger,
) http.RoundTripper {
	return &backendRoundTripper{
		transport: transport,
		iter:      iter,
		after:     after,
		logger:    logger,
	}
}

func (rt *backendRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
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
		rt.logger.Error("endpoint-failed", err)
	}

	if rt.after != nil {
		rt.after(res, endpoint, err)
	}

	return res, err
}

func (rt *backendRoundTripper) selectEndpoint(request *http.Request) (*route.Endpoint, error) {
	endpoint := rt.iter.Next()
	if endpoint == nil {
		return nil, handler.NoEndpointsAvailable
	}

	rt.logger = rt.logger.WithData(lager.Data{"route-endpoint": endpoint.ToLogData()})
	return endpoint, nil
}

func (rt *backendRoundTripper) setupRequest(request *http.Request, endpoint *route.Endpoint) {
	rt.logger.Debug("backend")
	request.URL.Host = endpoint.CanonicalAddr()
	request.Header.Set("X-CF-ApplicationID", endpoint.ApplicationId)
	handler.SetRequestXCfInstanceId(request, endpoint)
}

func (rt *backendRoundTripper) reportError(err error) {
	rt.iter.EndpointFailed()
	rt.logger.Error("backend-endpoint-failed", err)
}
