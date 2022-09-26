package auth_test

import (
	"crypto/rand"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/log-cache/internal/auth"

	"code.cloudfoundry.org/log-cache/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessLog", func() {
	var (
		req       *http.Request
		timestamp time.Time
		al        *auth.AccessLog

		// request data
		method     string
		path       string
		url        string
		sourceHost string
		sourcePort string
		remoteAddr string
		dstHost    string
		dstPort    string

		forwardedFor string
		requestId    string
	)

	BeforeEach(func() {
		req = nil
		timestamp = time.Now()

		method = "GET"

		getRandomNumber := func() int {
			nBig, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
			if err != nil {
				log.Fatalf("Cannot generate random number %v", err)
			}
			return int(nBig.Int64())
		}

		path = fmt.Sprintf("/some/path?with_query=params-%d", getRandomNumber())
		url = "http://example.com" + path
		sourceHost = fmt.Sprintf("10.0.1.%d", getRandomNumber()%256)
		sourcePort = strconv.Itoa(getRandomNumber()%65535 + 1)
		remoteAddr = sourceHost + ":" + sourcePort
		dstHost = fmt.Sprintf("10.1.2.%d", getRandomNumber()%256)
		dstPort = strconv.Itoa(getRandomNumber()%65535 + 1)

		forwardedFor = fmt.Sprintf("10.0.0.%d", getRandomNumber()%256)
		requestId = fmt.Sprintf("test-vcap-request-id-%d", getRandomNumber())
	})

	JustBeforeEach(func() {
		req = testing.BuildRequest(method, url, remoteAddr, requestId, forwardedFor)
		al = auth.NewAccessLog(req, timestamp, dstHost, dstPort)
	})

	Describe("String", func() {
		Context("with a GET request", func() {
			BeforeEach(func() {
				method = "GET"
			})

			It("returns a log with GET as the method", func() {
				expected := testing.BuildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					forwardedFor,
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with a POST request", func() {
			BeforeEach(func() {
				method = "POST"
			})

			It("returns a log with POST as the method", func() {
				expected := testing.BuildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					forwardedFor,
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with X-Forwarded-For not set", func() {
			BeforeEach(func() {
				forwardedFor = ""
			})

			It("uses remoteAddr", func() {
				expected := testing.BuildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					sourceHost,
					sourcePort,
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with X-Forwarded-For containing multiple values", func() {
			BeforeEach(func() {
				forwardedFor = "123.22.11.1, 6.3.4.5, 1.2.3.4"
			})

			It("uses remoteAddr", func() {
				expected := testing.BuildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					"123.22.11.1",
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with a request that has no query params", func() {
			BeforeEach(func() {
				path = "/some/path"
				url = "http://example.com" + path
			})

			It("writes log without question mark delimiter", func() {
				prefix := "CEF:0|cloud_foundry|log_cache|1.0|GET /some/path|"
				Expect(al.String()).To(HavePrefix(prefix))
			})
		})
	})
})
