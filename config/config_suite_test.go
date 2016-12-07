package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/uber-go/zap"

	"testing"
)

var (
	logger zap.Logger
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = BeforeEach(func() {
	logger = zap.New(zap.NewJSONEncoder())
})
