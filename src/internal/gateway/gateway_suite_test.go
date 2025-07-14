package gateway_test

import (
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGateway(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, nil)))

	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway Suite")
}
