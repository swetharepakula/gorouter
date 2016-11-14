package metrics

import (
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/workpool"

	"code.cloudfoundry.org/gorouter/metrics/reporter"
	"code.cloudfoundry.org/gorouter/route"
)

type MetricsProcessor struct {
	rep  reporter.Reporter
	pool *workpool.WorkPool
}

func NewMetricsProcessor(reporter reporter.Reporter, maxWorkers int) (*MetricsProcessor, error) {
	pool, err := workpool.NewWorkPool(maxWorkers)
	if err != nil {
		return nil, err
	}

	return &MetricsProcessor{
		rep:  reporter,
		pool: pool,
	}, nil
}

func (p *MetricsProcessor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	select {
	case <-signals:
		p.pool.Stop()
		return nil
	}
}

func (p *MetricsProcessor) CaptureBadRequest(req *http.Request) {
	p.pool.Submit(func() {
		p.rep.CaptureBadRequest(req)
	})
}

func (p *MetricsProcessor) CaptureBadGateway(req *http.Request) {
	p.pool.Submit(func() {
		p.rep.CaptureBadGateway(req)
	})
}

func (p *MetricsProcessor) CaptureRoutingRequest(b *route.Endpoint, req *http.Request) {
	p.pool.Submit(func() {
		p.rep.CaptureRoutingRequest(b, req)
	})
}

func (p *MetricsProcessor) CaptureRoutingResponse(b *route.Endpoint, res *http.Response, t time.Time, d time.Duration) {
	p.pool.Submit(func() {
		p.rep.CaptureRoutingResponse(b, res, t, d)
	})
}

func (p *MetricsProcessor) CaptureRouteStats(totalRoutes int, msSinceLastUpdate uint64) {
	p.pool.Submit(func() {
		p.rep.CaptureRouteStats(totalRoutes, msSinceLastUpdate)
	})
}

func (p *MetricsProcessor) CaptureLookupTime(t time.Duration) {
	p.pool.Submit(func() {
		p.rep.CaptureLookupTime(t)
	})
}

func (p *MetricsProcessor) CaptureRegistryMessage(msg reporter.ComponentTagged) {
	p.pool.Submit(func() {
		p.rep.CaptureRegistryMessage(msg)
	})
}
