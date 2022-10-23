package promql_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPromql(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Promql Suite")
}
