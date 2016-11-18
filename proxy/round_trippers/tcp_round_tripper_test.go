package round_trippers_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/proxy/round_trippers"
	"code.cloudfoundry.org/gorouter/proxy/round_trippers/fakes"
	"code.cloudfoundry.org/gorouter/route"
	routefakes "code.cloudfoundry.org/gorouter/route/fakes"
	"code.cloudfoundry.org/gorouter/test_util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/routing-api/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TcpRoundTripper", func() {
	var (
		proxyRoundTripper http.RoundTripper
		logger            lager.Logger
		endpointIterator  *routefakes.FakeEndpointIterator
		after             round_trippers.AfterRoundTrip
		req               *http.Request
		hijacker          *fakes.FakeHijacker
		backendListener   net.Listener
		clientConn        *fakes.FakeConn
		endpoint          *route.Endpoint
	)

	BeforeEach(func() {
		var err error

		// Listen for incoming connections.
		backendListener, err = net.Listen("tcp", "localhost:0")
		Expect(err).NotTo(HaveOccurred())
		go launchServer(backendListener)

		req = test_util.NewRequest("GET", "tcp-handler", "/chat", nil)
		req.Header.Set("Upgrade", "tcp")

		req.Header.Set("Connection", "UpgradE")

		endpointIterator = &routefakes.FakeEndpointIterator{}
		logger = lagertest.NewTestLogger("test")

		host, port, err := net.SplitHostPort(backendListener.Addr().String())
		Expect(err).NotTo(HaveOccurred())

		portNum, err := strconv.Atoi(port)
		Expect(err).NotTo(HaveOccurred())
		endpoint = route.NewEndpoint("", host, uint16(portNum), "", "", nil, 1000, "", models.ModificationTag{})

		hijacker = &fakes.FakeHijacker{}
		clientConn = &fakes.FakeConn{}
		finishedReading := false
		clientConn.ReadStub = func(buf []byte) (int, error) {
			if finishedReading {
				return 0, io.EOF
			}
			if len(buf) < 4 {
				return 0, errors.New("buffer too small")
			}

			buf[0] = 0
			buf[1] = 1
			buf[2] = 2
			buf[3] = 3
			return 4, nil
		}

		hijacker.HijackReturns(clientConn, nil, err)
		endpointIterator.NextReturns(endpoint)
	})

	JustBeforeEach(func() {
		proxyRoundTripper = round_trippers.NewTcpRoundTripper(hijacker, endpointIterator, after, logger)
	})

	AfterEach(func() {
		backendListener.Close()
	})

	It("downgrades the http request to a tcp connection", func() {
		resp, err := proxyRoundTripper.RoundTrip(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))

		Expect(hijacker.HijackCallCount()).To(Equal(1))
		Expect(endpointIterator.NextCallCount()).To(Equal(1))
		Expect(clientConn.ReadCallCount()).To(BeNumerically(">=", 1))
		Expect(clientConn.WriteCallCount()).To(BeNumerically(">=", 1))
		Expect(clientConn.WriteArgsForCall(0)).To(Equal([]byte{0, 1, 2, 3}))

		Expect(clientConn.CloseCallCount()).To(Equal(1))
	})

	Context("when the endpoint fails", func() {
		var numTries int
		BeforeEach(func() {
			newEndpoint := route.NewEndpoint("", "[127.0.1.1", uint16(6893), "", "", nil, 1000, "", models.ModificationTag{})
			numTries = 0
			endpointIterator.NextStub = func() *route.Endpoint {
				numTries++
				if numTries <= 2 {
					return newEndpoint
				}
				return endpoint
			}
		})

		It("retries", func() {
			resp, err := proxyRoundTripper.RoundTrip(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))

			Expect(hijacker.HijackCallCount()).To(Equal(1))
			Expect(endpointIterator.NextCallCount()).To(Equal(3))
			Expect(clientConn.ReadCallCount()).To(BeNumerically(">=", 1))
			Expect(clientConn.WriteCallCount()).To(BeNumerically(">=", 1))
			Expect(clientConn.WriteArgsForCall(0)).To(Equal([]byte{0, 1, 2, 3}))

			Expect(clientConn.CloseCallCount()).To(Equal(1))
			Expect(numTries).To(Equal(3))
		})
	})

	Context("when there is no endpoint available", func() {
		var afterCalled bool

		BeforeEach(func() {
			endpointIterator.NextReturns(nil)
			after = func(rsp *http.Response, endpoint *route.Endpoint, err error) {
				Expect(err).To(Equal(handler.NoEndpointsAvailable))
				afterCalled = true
			}
		})

		It("fails with an error", func() {
			_, err := proxyRoundTripper.RoundTrip(req)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(handler.NoEndpointsAvailable))

			Expect(afterCalled).To(BeTrue())
		})
	})

	Context("when it cannot hijack the connection", func() {
		var afterCalled bool
		BeforeEach(func() {
			hijacker.HijackReturns(nil, nil, errors.New("you got an error"))
			after = func(rsp *http.Response, endpoint *route.Endpoint, err error) {
				Expect(err).To(HaveOccurred())
				afterCalled = true
			}
		})

		It("returns the error", func() {
			_, err := proxyRoundTripper.RoundTrip(req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("you got an error"))
			Expect(afterCalled).To(BeTrue())
		})
	})
})

func launchServer(listener net.Listener) {
	defer GinkgoRecover()
	// Listen for an incoming connection.
	conn, err := listener.Accept()
	if err != nil {
		return
	}

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	defer conn.Close()
	// Make a buffer to hold incoming data.
	buff := make([]byte, 1024)
	readBytes, err := conn.Read(buff)
	Expect(err).NotTo(HaveOccurred())

	var writeBuffer bytes.Buffer
	writeBuffer.Write(buff[0:readBytes])
	_, err = conn.Write(writeBuffer.Bytes())
	Expect(err).NotTo(HaveOccurred())
}
