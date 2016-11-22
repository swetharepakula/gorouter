package registry

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/gorouter/config"
	"code.cloudfoundry.org/gorouter/metrics/reporter"
	"code.cloudfoundry.org/gorouter/registry/container"
	"code.cloudfoundry.org/gorouter/route"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/models"
)

//go:generate counterfeiter -o fakes/fake_registry_interface.go . RegistryInterface
type RegistryInterface interface {
	Register(uri route.Uri, endpoint *route.Endpoint)
	Unregister(uri route.Uri, endpoint *route.Endpoint)
	Lookup(uri route.Uri) *route.Pool
	LookupWithInstance(uri route.Uri, appId, appIndex string) *route.Pool
	StartPruningCycle()
	StopPruningCycle()
	NumUris() int
	NumEndpoints() int
	MarshalJSON() ([]byte, error)
}

type PruneStatus int

const (
	CONNECTED = PruneStatus(iota)
	DISCONNECTED
)

type RouteRegistry struct {
	sync.RWMutex

	logger lager.Logger

	// Access to the Trie datastructure should be governed by the RWMutex of RouteRegistry
	byUri *container.Trie

	// used for ability to suspend pruning
	suspendPruning func() bool
	pruningStatus  PruneStatus

	pruneStaleDropletsInterval time.Duration
	dropletStaleThreshold      time.Duration

	reporter reporter.RouteRegistryReporter

	ticker           *time.Ticker
	timeOfLastUpdate time.Time
	pool             *route.Pool
}

func NewRouteRegistry(logger lager.Logger, c *config.Config, reporter reporter.RouteRegistryReporter) *RouteRegistry {
	r := &RouteRegistry{}
	r.logger = logger
	r.byUri = container.NewTrie()

	r.pruneStaleDropletsInterval = c.PruneStaleDropletsInterval
	r.dropletStaleThreshold = c.DropletStaleThreshold
	r.suspendPruning = func() bool { return false }

	pool := route.NewPool(10*time.Second, "")
	endpoint := route.NewEndpoint("", "gostatic-0.foo.com", 8080, "", "", nil, 120, "", models.ModificationTag{})
	pool.Put(endpoint)
	r.pool = pool

	r.reporter = reporter
	return r
}

func (r *RouteRegistry) Register(uri route.Uri, endpoint *route.Endpoint) {
}

func (r *RouteRegistry) Unregister(uri route.Uri, endpoint *route.Endpoint) {
}

func (r *RouteRegistry) Lookup(uri route.Uri) *route.Pool {
	return r.pool
}

func (r *RouteRegistry) LookupWithInstance(uri route.Uri, appId string, appIndex string) *route.Pool {
	return r.Lookup(uri)
}

func (r *RouteRegistry) StartPruningCycle() {
}

func (r *RouteRegistry) StopPruningCycle() {
}

func (registry *RouteRegistry) NumUris() int {
	return 1
}

func (r *RouteRegistry) TimeOfLastUpdate() time.Time {
	return time.Now()
}

func (r *RouteRegistry) NumEndpoints() int {
	return 1
}

func (r *RouteRegistry) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.byUri.ToMap())
}

func (r *RouteRegistry) pruneStaleDroplets() {
}

func (r *RouteRegistry) SuspendPruning(f func() bool) {
	r.Lock()
	r.suspendPruning = f
	r.Unlock()
}

// bulk update to mark pool / endpoints as updated
func (r *RouteRegistry) freshenRoutes() {
}

func parseContextPath(uri route.Uri) string {
	contextPath := "/"
	split := strings.SplitN(strings.TrimPrefix(uri.String(), "/"), "/", 2)

	if len(split) > 1 {
		contextPath += split[1]
	}
	return contextPath
}
