package metrics_test

import (
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/gorouter/metrics"
	"code.cloudfoundry.org/gorouter/metrics/reporter/fakes"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/routing-api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics Processor", func() {
	var (
		processor    *metrics.MetricsProcessor
		fakeReporter *fakes.FakeReporter
		endpoint     *route.Endpoint
		resp         *http.Response
		req          *http.Request
	)

	BeforeEach(func() {
		var err error
		fakeReporter = &fakes.FakeReporter{}
		processor, err = metrics.NewMetricsProcessor(fakeReporter, 1)
		Expect(err).NotTo(HaveOccurred())

		modTag := models.ModificationTag{}
		endpoint = route.NewEndpoint("id", "host", 8888, "privateid", "index", nil, 10, "rs", modTag)
		resp = &http.Response{}
		req = &http.Request{}
	})

	It("sends CaptureBadRequest messages", func() {
		processor.CaptureBadRequest(req)
		Eventually(fakeReporter.CaptureBadRequestCallCount).Should(Equal(1))
	})

	It("sends CaptureBadGateway messages", func() {
		processor.CaptureBadGateway(req)
		Eventually(fakeReporter.CaptureBadGatewayCallCount).Should(Equal(1))
	})

	It("sends CaptureRoutingResponse messages", func() {
		processor.CaptureRoutingResponse(endpoint, resp, time.Now(), time.Second)
		Eventually(fakeReporter.CaptureRoutingResponseCallCount).Should(Equal(1))
	})

	It("sends CaptureRoutingRequest messages", func() {
		processor.CaptureRoutingRequest(endpoint, req)
		Eventually(fakeReporter.CaptureRoutingRequestCallCount).Should(Equal(1))
	})

	It("sends CaptureRouteStats messages", func() {
		processor.CaptureRouteStats(1, 2)
		Eventually(fakeReporter.CaptureRouteStatsCallCount).Should(Equal(1))
	})

	It("sends CaptureLookupTime messages", func() {
		processor.CaptureLookupTime(time.Second)
		Eventually(fakeReporter.CaptureLookupTimeCallCount).Should(Equal(1))
	})

	It("sends CaptureRegistryMessage messages", func() {
		processor.CaptureRegistryMessage(endpoint)
		Eventually(fakeReporter.CaptureRegistryMessageCallCount).Should(Equal(1))
	})

	It("stops when signaled", func() {
		signals := make(chan os.Signal, 1)
		ready := make(chan struct{})

		signals <- os.Interrupt
		Expect(processor.Run(signals, ready)).To(Succeed())
		Expect(ready).To(BeClosed())
	})

	Context("when invalid maxWorkers value is passed", func() {
		It("returns an error", func() {
			_, err := metrics.NewMetricsProcessor(fakeReporter, -1)
			Expect(err).To(HaveOccurred())
		})
	})
})
