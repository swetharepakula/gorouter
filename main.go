package main

import (
	"crypto/tls"
	"net/url"
	"sync/atomic"

	"code.cloudfoundry.org/cflager"
	"code.cloudfoundry.org/gorouter/common/secure"
	"code.cloudfoundry.org/gorouter/config"
	"code.cloudfoundry.org/gorouter/proxy"
	rregistry "code.cloudfoundry.org/gorouter/registry"
	"code.cloudfoundry.org/gorouter/router"
	rvarz "code.cloudfoundry.org/gorouter/varz"
	"github.com/nats-io/nats"

	"flag"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFile string

var healthCheck int32

const (
	DEBUG = "debug"
	INFO  = "info"
	ERROR = "error"
	FATAL = "fatal"
)

func main() {
	flag.StringVar(&configFile, "c", "", "Configuration File")
	cflager.AddFlags(flag.CommandLine)
	flag.Parse()

	c := config.DefaultConfig()

	// setup number of procs
	if c.GoMaxProcs != 0 {
		runtime.GOMAXPROCS(c.GoMaxProcs)
	}

	// if c.DebugAddr != "" {
	// 	debugserver.Run(c.DebugAddr, reconfigurableSink)
	// }

	natsClient, natsHost := connectToNatsServer(c)

	registry := rregistry.NewRouteRegistry(c)
	if c.SuspendPruningIfNatsUnavailable {
		registry.SuspendPruning(func() bool { return !(natsClient.Status() == nats.CONNECTED) })
	}

	varz := rvarz.NewVarz(registry)

	//	accessLogger, err := access_log.CreateRunningAccessLogger(logger.Session("access-log"), c)

	var crypto secure.Crypto
	var cryptoPrev secure.Crypto
	if c.RouteServiceEnabled {
		crypto = createCrypto(c.RouteServiceSecret)
		if c.RouteServiceSecretPrev != "" {
			cryptoPrev = createCrypto(c.RouteServiceSecretPrev)
		}
	}

	proxy := buildProxy(c, registry, crypto, cryptoPrev)
	healthCheck = 0
	router, err := router.NewRouter(c, proxy, natsClient, registry, varz, &healthCheck, nil)
	router.NatsHost = natsHost

	members := grouper.Members{
		{"router", router},
	}
	// if c.RoutingApiEnabled() {
	// 	routeFetcher := setupRouteFetcher(c, registry)

	// 	// check connectivity to routing api
	// 	err = routeFetcher.FetchRoutes()
	// 	members = append(members, grouper.Member{Name: "router-fetcher", Runner: routeFetcher})
	// }

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1))

	err = <-monitor.Wait()
	if err != nil {
		os.Exit(1)
	}

	os.Exit(0)
}

func createCrypto(secret string) *secure.AesGCM {
	// generate secure encryption key using key derivation function (pbkdf2)
	secretPbkdf2 := secure.NewPbkdf2([]byte(secret), 16)
	crypto, _ := secure.NewAesGCM(secretPbkdf2)
	return crypto
}

func buildProxy(c *config.Config, registry rregistry.RegistryInterface, crypto secure.Crypto, cryptoPrev secure.Crypto) proxy.Proxy {
	args := proxy.ProxyArgs{
		EndpointTimeout: c.EndpointTimeout,
		Ip:              c.Ip,
		TraceKey:        c.TraceKey,
		Registry:        registry,
		SecureCookies:   c.SecureCookies,
		TLSConfig: &tls.Config{
			CipherSuites:       c.CipherSuites,
			InsecureSkipVerify: c.SkipSSLValidation,
		},
		RouteServiceEnabled:        c.RouteServiceEnabled,
		RouteServiceTimeout:        c.RouteServiceTimeout,
		RouteServiceRecommendHttps: c.RouteServiceRecommendHttps,
		Crypto:                   crypto,
		CryptoPrev:               cryptoPrev,
		ExtraHeadersToLog:        &c.ExtraHeadersToLog,
		HealthCheckUserAgent:     c.HealthCheckUserAgent,
		HeartbeatOK:              &healthCheck,
		EnableZipkin:             c.Tracing.EnableZipkin,
		ForceForwardedProtoHttps: c.ForceForwardedProtoHttps,
		DefaultLoadBalance:       c.LoadBalance,
	}
	return proxy.NewProxy(args)
}

// func setupRouteFetcher(, c *config.Config, registry rregistry.RegistryInterface) *route_fetcher.RouteFetcher {
// 	clock := clock.NewClock()

// 	uaaClient := newUaaClient(logger, clock, c)

// 	_, err := uaaClient.FetchToken(true)

// 	routingApiUri := fmt.Sprintf("%s:%d", c.RoutingApi.Uri, c.RoutingApi.Port)
// 	routingApiClient := routing_api.NewClient(routingApiUri, false)

// 	routeFetcher := route_fetcher.NewRouteFetcher(logger, uaaClient, registry, c, routingApiClient, 1, clock)
// 	return routeFetcher
// }

// func newUaaClient(logger lager.Logger, clock clock.Clock, c *config.Config) uaa_client.Client {
// 	if c.RoutingApi.AuthDisabled {
// 		return uaa_client.NewNoOpUaaClient()
// 	}
// 	tokenURL := fmt.Sprintf("https://%s:%d", c.OAuth.TokenEndpoint, c.OAuth.Port)

// 	cfg := &uaa_config.Config{
// 		UaaEndpoint:           tokenURL,
// 		SkipVerification:      c.OAuth.SkipSSLValidation,
// 		ClientName:            c.OAuth.ClientName,
// 		ClientSecret:          c.OAuth.ClientSecret,
// 		CACerts:               c.OAuth.CACerts,
// 		MaxNumberOfRetries:    c.TokenFetcherMaxRetries,
// 		RetryInterval:         c.TokenFetcherRetryInterval,
// 		ExpirationBufferInSec: c.TokenFetcherExpirationBufferTimeInSeconds,
// 	}

// 	uaaClient, err := uaa_client.NewClient(logger, cfg, clock)
// 	if err != nil {
// 		logger.Fatal("initialize-token-fetcher-error", err)
// 	}
// 	return uaaClient
// }

func connectToNatsServer(c *config.Config) (*nats.Conn, *atomic.Value) {
	var natsClient *nats.Conn
	var natsHost atomic.Value
	var err error

	natsServers := c.NatsServers()
	attempts := 3
	for attempts > 0 {
		options := nats.DefaultOptions
		options.Servers = natsServers
		options.PingInterval = c.NatsClientPingInterval

		// in the case of suspending pruning, we need to ensure we retry reconnects indefinitely
		if c.SuspendPruningIfNatsUnavailable {
			options.MaxReconnect = -1
		}
		natsClient, err = options.Connect()
		if err == nil {
			break
		} else {
			attempts--
			time.Sleep(100 * time.Millisecond)
		}
	}

	var natsHostStr string
	natsUrl, err := url.Parse(natsClient.ConnectedUrl())
	if err == nil {
		natsHostStr = natsUrl.Host
	}

	natsHost.Store(natsHostStr)
	return natsClient, &natsHost
}

// func InitLoggerFromConfig(logger lager.Logger, c *config.Config, logCounter *schema.LogCounter) {
// 	if c.Logging.File != "" {
// 		file, err := os.OpenFile(c.Logging.File, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
// 		if err != nil {
// 			logger.Fatal("error-opening-log-file", err, lager.Data{"file": c.Logging.File})
// 		}
// 		var logLevel lager.LogLevel
// 		switch c.Logging.Level {
// 		case DEBUG:
// 			logLevel = lager.DEBUG
// 		case INFO:
// 			logLevel = lager.INFO
// 		case ERROR:
// 			logLevel = lager.ERROR
// 		case FATAL:
// 			logLevel = lager.FATAL
// 		default:
// 			panic(fmt.Errorf("unknown log level: %s", c.Logging.Level))
// 		}
// 		logger.RegisterSink(lager.NewWriterSink(file, logLevel))
// 	}

// 	logger.RegisterSink(logCounter)
// }
