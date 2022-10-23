package auth_test

import (
	"errors"

	"code.cloudfoundry.org/log-cache/internal/auth"

	"code.cloudfoundry.org/log-cache/internal/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DefaultAccessLogger", func() {
	var (
		writer *spyWriter
		logger *auth.DefaultAccessLogger
	)

	BeforeEach(func() {
		writer = &spyWriter{}
		logger = auth.NewAccessLogger(writer)
	})

	It("logs Access", func() {
		req, err := testing.NewServerRequest("GET", "some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(logger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		prefix := "CEF:0|cloud_foundry|log_cache|1.0|GET some.url.com/foo|GET some.url.com/foo|0|"
		Expect(writer.message).To(HavePrefix(prefix))
	})

	It("includes details about the access", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"

		Expect(logger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		Expect(writer.message).To(ContainSubstring("src=127.0.0.1 spt=4567"))
	})

	It("uses X-Forwarded-For if it exists", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"
		req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

		Expect(logger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		Expect(writer.message).To(ContainSubstring("src=50.60.70.80 spt=1234"))
	})

	It("writes multiple log lines", func() {
		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"

		Expect(logger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		expected := "src=127.0.0.1 spt=4567"
		Expect(writer.message).To(ContainSubstring(expected))
		Expect(writer.message).To(HaveSuffix("\n"))

		req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

		Expect(logger.LogAccess(req, "1.1.1.1", "1")).To(Succeed())
		expected = "src=50.60.70.80 spt=1234"
		Expect(writer.message).To(ContainSubstring(expected))
		Expect(writer.message).To(HaveSuffix("\n"))
	})

	It("returns an error", func() {
		writer = &spyWriter{}
		writer.err = errors.New("boom")
		logger = auth.NewAccessLogger(writer)

		req, err := testing.NewServerRequest("GET", "http://some.url.com/foo", nil)
		Expect(err).ToNot(HaveOccurred())
		req.RemoteAddr = "127.0.0.1:4567"
		Expect(logger.LogAccess(req, "1.1.1.1", "1")).ToNot(Succeed())
	})
})

type spyWriter struct {
	message []byte
	err     error
}

func (s *spyWriter) Write(message []byte) (sent int, err error) {
	s.message = message
	return 0, s.err
}
