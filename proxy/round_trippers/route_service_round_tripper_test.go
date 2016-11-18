package round_trippers_test

import (
	"errors"
	"net"
	"net/http"

	"code.cloudfoundry.org/gorouter/proxy/round_trippers"
	roundtripperfakes "code.cloudfoundry.org/gorouter/proxy/round_trippers/fakes"
	proxyfakes "code.cloudfoundry.org/gorouter/proxy/utils/fakes"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/gorouter/route_service"
	"code.cloudfoundry.org/gorouter/test_util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RouteServiceRoundTripper", func() {
	Context("RoundTrip", func() {
		var (
			proxyRoundTripper http.RoundTripper
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
			req = test_util.NewRequest("GET", "myapp.com", "/", nil)
			req.URL.Scheme = "http"
			resp = &proxyfakes.FakeProxyResponseWriter{}

			logger = lagertest.NewTestLogger("test")
			transport = &roundtripperfakes.FakeRoundTripper{}
			req.Header.Set(route_service.RouteServiceForwardedUrl, "http://myapp.com/")

			after = func(rsp *http.Response, endpoint *route.Endpoint, err error) {
				Expect(endpoint.Tags).ShouldNot(BeNil())
			}
			proxyRoundTripper = round_trippers.NewRouteServiceRoundTripper(transport, after, logger)
		})

		Context("when the first request to the route service fails", func() {
			BeforeEach(func() {
				firstCall := true

				transport.RoundTripStub = func(req *http.Request) (*http.Response, error) {
					var err error

					err = nil
					if firstCall {
						err = dialError
					}
					firstCall = false

					return nil, err
				}
			})

			It("does not set X-CF-Forwarded-Url to the route service URL", func() {
				_, err := proxyRoundTripper.RoundTrip(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(req.Header.Get(route_service.RouteServiceForwardedUrl)).To(Equal("http://myapp.com/"))
			})

		})

		Context("when the route service is not available", func() {
			var roundTripCallCount int

			BeforeEach(func() {
				transport.RoundTripStub = func(req *http.Request) (*http.Response, error) {
					roundTripCallCount++
					return nil, dialError
				}
			})

			It("retries 3 times", func() {
				_, err := proxyRoundTripper.RoundTrip(req)
				Expect(err).To(HaveOccurred())
				Expect(roundTripCallCount).To(Equal(3))
			})
		})
	})
})
