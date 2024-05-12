package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Gateway", func() {
	var (
		port    int
		session *gexec.Session
		envVars map[string]string
		flc     *FakeLogCache
	)

	BeforeEach(func() {
		flc = NewFakeLogCache(fmt.Sprintf("localhost:%d", 8000+GinkgoParallelProcess()), nil)

		port = 8081 + GinkgoParallelProcess()
		envVars = map[string]string{
			"ADDR":           fmt.Sprintf(":%d", port),
			"LOG_CACHE_ADDR": flc.addr,
		}

		command := exec.Command(pathToGateway)
		for k, v := range envVars {
			command.Env = append(command.Env, fmt.Sprintf("%s=%s", k, v))
		}
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		flc.Start()
	})

	AfterEach(func() {
		session.Interrupt().Wait(2 * time.Second)
		flc.Stop()
	})

	It("serves requests", func() {
		var resp *http.Response
		Eventually(func() error {
			var err error
			resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/v1/info", port))
			return err
		}, "5s").ShouldNot(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
