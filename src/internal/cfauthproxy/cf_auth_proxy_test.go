package cfauthproxy_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"code.cloudfoundry.org/log-cache/internal/auth"
	. "code.cloudfoundry.org/log-cache/internal/cfauthproxy"
	"code.cloudfoundry.org/log-cache/internal/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuthProxy", func() {
	It("secure proxy proxies requests to a secure log cache gateway", func() {
		gateway := startSecureGateway("Hello World!")
		defer gateway.Close()

		proxy := newSecureCFAuthProxy(gateway.URL)
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeTLSReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})

	It("doesn't start the api until the readychecker returns nil", func() {
		gateway := startSecureGateway("Hello World!")
		defer gateway.Close()

		isReady := make(chan struct{})
		almostReadyChecker := func() error {
			select {
			case <-isReady:
				return nil
			default:
				return errors.New("Not Ready")
			}
		}

		proxy := newSecureCFAuthProxy(gateway.URL)
		defer proxy.Stop()
		go proxy.Start(almostReadyChecker)

		Consistently(proxy.Addr, time.Second).Should(BeEmpty())

		close(isReady)
		Eventually(proxy.Addr).ShouldNot(BeEmpty())

		resp, err := makeTLSReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})

	It("insecure proxy proxies requests to an insecure log cache gateway", func() {
		gateway := startGateway("Hello World!")
		defer gateway.Close()

		proxy := newInsecureCFAuthProxy(gateway.URL)
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})

	It("returns an error when proxying requests to an insecure log cache gateway", func() {
		// suppress tls error in test
		log.SetOutput(ioutil.Discard)

		testServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Write([]byte("Insecure Gateway not allowed"))
			}))
		defer testServer.Close()

		testGatewayURL, _ := url.Parse(testServer.URL)
		testGatewayURL.Scheme = "https"

		proxy := newSecureCFAuthProxy(testGatewayURL.String())
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeTLSReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
	})

	It("delegates to the auth middleware", func() {
		var middlewareCalled bool
		middleware := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			middlewareCalled = true
			w.WriteHeader(http.StatusNotFound)
		})

		proxy := newSecureCFAuthProxy(
			"https://127.0.0.1",
			WithAuthMiddleware(func(http.Handler) http.Handler {
				return middleware
			}),
		)
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeTLSReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		Expect(middlewareCalled).To(BeTrue())
	})

	It("delegates to the access middleware", func() {
		var middlewareCalled bool
		middleware := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			middlewareCalled = true
			w.WriteHeader(http.StatusNotFound)
		})

		proxy := newSecureCFAuthProxy(
			"https://127.0.0.1",
			WithAccessMiddleware(func(http.Handler) *auth.AccessHandler {
				return auth.NewAccessHandler(middleware, auth.NewNullAccessLogger(), "0.0.0.0", "1234")
			}),
		)
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeTLSReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		Expect(middlewareCalled).To(BeTrue())
	})

	It("does not accept unencrypted connections when configured for TLS", func() {
		testServer := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}),
		)
		proxy := newSecureCFAuthProxy(testServer.URL)
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeReq(proxy.Addr())
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("accepts plain text when TLS is disabled", func() {
		gateway := startSecureGateway("Hello World!")
		defer gateway.Close()

		proxy := newSecureCFAuthProxy(gateway.URL, WithCFAuthProxyTLSDisabled())
		defer proxy.Stop()
		startProxy(proxy, alwaysReadyChecker)

		resp, err := makeReq(proxy.Addr())
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, _ := ioutil.ReadAll(resp.Body)
		Expect(body).To(Equal([]byte("Hello World!")))
	})
})

var localhostCerts = testing.GenerateCerts("localhost-ca")

func alwaysReadyChecker() error {
	return nil
}

func newSecureCFAuthProxy(gatewayURL string, opts ...CFAuthProxyOption) *CFAuthProxy {
	parsedURL, err := url.Parse(gatewayURL)
	if err != nil {
		panic("couldn't parse gateway URL")
	}

	opts = append(opts, WithCFAuthProxyCACertPool(localhostCerts.Pool()))
	opts = append(opts, WithCFAuthProxyReadyCheckInterval(100*time.Millisecond))
	opts = append(opts, WithCFAuthProxyTLSServer(localhostCerts.Cert("localhost"), localhostCerts.Key("localhost")))
	return NewCFAuthProxy(
		parsedURL.String(),
		"127.0.0.1:0",
		opts...,
	)
}

func startProxy(proxy *CFAuthProxy, readyChecker func() error) {
	go proxy.Start(readyChecker)
	for proxy.Addr() == "" {
		time.Sleep(100 * time.Millisecond)
	}
}

func newInsecureCFAuthProxy(gatewayURL string, opts ...CFAuthProxyOption) *CFAuthProxy {
	parsedURL, err := url.Parse(gatewayURL)
	if err != nil {
		panic("couldn't parse gateway URL")
	}

	opts = append(opts, WithCFAuthProxyTLSDisabled())
	return NewCFAuthProxy(
		parsedURL.String(),
		"127.0.0.1:0",
		opts...,
	)
}

func startSecureGateway(responseBody string) *httptest.Server {
	testGateway := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte(responseBody))
		}),
	)

	cert, err := tls.LoadX509KeyPair(localhostCerts.Cert("localhost"), localhostCerts.Key("localhost"))
	if err != nil {
		panic(err)
	}

	testGateway.TLS = &tls.Config{
		RootCAs:      localhostCerts.Pool(),
		Certificates: []tls.Certificate{cert},
	}

	testGateway.StartTLS()

	return testGateway
}

func startGateway(responseBody string) *httptest.Server {
	testGateway := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte(responseBody))
		}),
	)

	testGateway.Start()

	return testGateway
}

func makeTLSReq(addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s", addr), nil)
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return client.Do(req)
}

func makeReq(addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s", addr), nil)
	Expect(err).ToNot(HaveOccurred())

	client := &http.Client{}

	return client.Do(req)
}
