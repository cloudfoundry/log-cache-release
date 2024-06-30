package integration_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type ComponentPaths struct {
	Gateway      string `json:"gateway_path"`
	CFAuthProxy  string `json:"cf_auth_proxy_path"`
	LogCache     string `json:"log_cache_path"`
	SyslogServer string `json:"syslog_server_path"`
}

func NewComponentPaths() ComponentPaths {
	cps := ComponentPaths{}

	path, err := gexec.Build("code.cloudfoundry.org/log-cache/cmd/gateway", "-ldflags", "-X main.buildVersion=1.2.3")
	Expect(err).NotTo(HaveOccurred())
	cps.Gateway = path

	path, err = gexec.Build("code.cloudfoundry.org/log-cache/cmd/cf-auth-proxy")
	Expect(err).NotTo(HaveOccurred())
	cps.CFAuthProxy = path

	path, err = gexec.Build("code.cloudfoundry.org/log-cache/cmd/log-cache")
	Expect(err).NotTo(HaveOccurred())
	cps.LogCache = path

	path, err = gexec.Build("code.cloudfoundry.org/log-cache/cmd/syslog-server")
	Expect(err).NotTo(HaveOccurred())
	cps.SyslogServer = path

	return cps
}

func (cps *ComponentPaths) Marshal() []byte {
	data, err := json.Marshal(cps)
	Expect(err).NotTo(HaveOccurred())
	return data
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var componentPaths ComponentPaths

var _ = SynchronizedBeforeSuite(func() []byte {
	cps := NewComponentPaths()
	return cps.Marshal()
}, func(data []byte) {
	Expect(json.Unmarshal(data, &componentPaths)).To(Succeed())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})
