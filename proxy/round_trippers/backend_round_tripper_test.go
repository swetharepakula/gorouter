package round_trippers_test

import (
	"errors"
	"net"
	"net/http"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/proxy/round_trippers"
	roundtripperfakes "code.cloudfoundry.org/gorouter/proxy/round_trippers/fakes"
	proxyfakes "code.cloudfoundry.org/gorouter/proxy/utils/fakes"
	"code.cloudfoundry.org/gorouter/route"
	routefakes "code.cloudfoundry.org/gorouter/route/fakes"
	"code.cloudfoundry.org/gorouter/test_util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BackendRoundTripper", func() {
	Context("RoundTrip", func() {
		var (
			proxyRoundTripper http.RoundTripper
			endpointIterator  *routefakes.FakeEndpointIterator
			transport         *roundtripperfakes.FakeRoundTripper
			logger            lager.Logger
			req               *http.Request
			resp              *proxyfakes.FakeProxyResponseWriter
			dialError         = &net.OpError{
				Err: errors.New("error"),
				Op:  "dial",
			}
			after round_trippers.AfterRoundTrip
		)

		BeforeEach(func() {
			endpointIterator = &routefakes.FakeEndpointIterator{}
			req = test_util.NewRequest("GET", "myapp.com", "/", nil)
			req.URL.Scheme = "http"
			resp = &proxyfakes.FakeProxyResponseWriter{}

			logger = lagertest.NewTestLogger("test")
			transport = &roundtripperfakes.FakeRoundTripper{}
			endpoint := &route.Endpoint{
				Tags: map[string]string{},
			}

			endpointIterator.NextReturns(endpoint)

			proxyRoundTripper = round_trippers.NewBackendRoundTripper(transport, endpointIterator, after, logger)
		})

		Context("when backend is unavailable", func() {
			BeforeEach(func() {
				transport.RoundTripStub = func(req *http.Request) (*http.Response, error) {
					return nil, dialError
				}
			})

			It("retries 3 times", func() {
				resp.HeaderReturns(make(http.Header))
				_, err := proxyRoundTripper.RoundTrip(req)
				Expect(err).To(HaveOccurred())
				Expect(endpointIterator.NextCallCount()).To(Equal(3))
			})
		})

		Context("when there are no more endpoints available", func() {
			BeforeEach(func() {
				endpointIterator.NextReturns(nil)
			})

			It("returns a 502 BadGateway error", func() {
				resp.HeaderReturns(make(http.Header))
				backendRes, err := proxyRoundTripper.RoundTrip(req)
				Expect(err).To(HaveOccurred())
				Expect(backendRes).To(BeNil())
				Expect(err).To(Equal(handler.NoEndpointsAvailable))
			})
		})

		Context("when the first request to the backend fails", func() {
			BeforeEach(func() {
				firstCall := true
				transport.RoundTripStub = func(req *http.Request) (*http.Response, error) {
					var err error
					err = nil
					if firstCall {
						err = dialError
						firstCall = false
					}
					return nil, err
				}
			})

			It("retries 3 times", func() {
				_, err := proxyRoundTripper.RoundTrip(req)
				Expect(err).ToNot(HaveOccurred())
				Expect(endpointIterator.NextCallCount()).To(Equal(2))
			})
		})
	})
})
