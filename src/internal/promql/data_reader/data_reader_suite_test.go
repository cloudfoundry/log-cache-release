package data_reader_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDataReader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DataReader Suite")
}
