package registry_test

import (
	"testing"
	"time"

	"code.cloudfoundry.org/gorouter/config"
	"code.cloudfoundry.org/gorouter/metrics/reporter/fakes"
	"code.cloudfoundry.org/gorouter/registry"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/routing-api/models"
	"github.com/uber-go/zap"
)

func BenchmarkRegistry(b *testing.B) {
	logger := zap.New(zap.NullEncoder())

	configObj := config.DefaultConfig()
	configObj.PruneStaleDropletsInterval = 50 * time.Millisecond
	configObj.DropletStaleThreshold = 24 * time.Millisecond
	reporter := new(fakes.FakeRouteRegistryReporter)
	r := registry.NewRouteRegistry(logger, configObj, reporter)

	modTag := models.ModificationTag{}
	fooEndpoint := route.NewEndpoint("12345", "192.168.1.1", 1234,
		"id1", "0",
		map[string]string{
			"runtime":   "ruby18",
			"framework": "sinatra",
		}, -1, "", modTag)

	for i := 0; i < b.N; i++ {
		r.Register("foo", fooEndpoint)
	}
}
