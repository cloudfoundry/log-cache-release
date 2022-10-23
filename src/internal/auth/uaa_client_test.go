package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/go-metric-registry/testhelpers"

	"code.cloudfoundry.org/log-cache/internal/auth"

	"bytes"
	"encoding/json"
	"encoding/pem"
	"net/http"

	jose "github.com/dvsekhvalnov/jose2go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAAClient", func() {
	Context("HS256", func() {
		It("accepts tokens that are signed with HS256", func() {
			tc := uaaSetup(false)
			tc.PrimePublicKeyCache(false)
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateHS256SignedToken(payload)

			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeTrue())
		})
	})
	Context("Read()", func() {
		var tc *UAATestContext

		BeforeEach(func() {
			tc = uaaSetup(true)
			tc.PrimePublicKeyCache(true)
		})

		It("only accepts tokens that are signed with RS256", func() {
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateUnsignedToken(payload)

			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to decode token: unsupported algorithm: none"))
		})

		It("returns IsAdmin == true when scopes include doppler.firehose", func() {
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)

			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeTrue())
		})

		It("returns IsAdmin == true when scopes include logs.admin", func() {
			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)

			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeTrue())
		})

		It("returns IsAdmin == false when scopes include neither logs.admin nor doppler.firehose", func() {
			payload := tc.BuildValidPayload("foo.bar")
			token := tc.CreateSignedToken(payload)

			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeFalse())
		})

		It("returns context with correct ExpiresAt", func() {
			t := time.Now().Add(time.Hour).Truncate(time.Second)
			payload := fmt.Sprintf(`{"scope":["logs.admin"], "exp":%d}`, t.Unix())
			token := tc.CreateSignedToken(payload)

			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.ExpiresAt).To(Equal(t))
		})

		It("does offline token validation", func() {
			initialRequestCount := len(tc.httpClient.requests)

			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)

			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())

			_, err = tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())

			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount))
		})

		It("does not allow use of an expired token", func() {
			tc.GenerateSingleTokenKeyResponse(true)

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())

			expiredPayload := tc.BuildExpiredPayload("logs.Admin")
			expiredToken := tc.CreateSignedToken(expiredPayload)

			_, err = tc.uaaClient.Read(withBearer(expiredToken))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token is expired"))
		})

		It("returns an error when token is blank", func() {
			_, err := tc.uaaClient.Read("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("missing token"))
		})

		Context("when a token is signed with a private key that is unknown", func() {
			It("validates the token successfully when the matching public key can be retrieved from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)
				newPrivateKey := generateLegitTokenKey("testKey2")

				tc.AddPrivateKeyToUAATokenKeyResponse(newPrivateKey)

				payload := tc.BuildValidPayload("logs.admin")
				token := tc.CreateSignedTokenUsingPrivateKey(payload, newPrivateKey)

				c, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Token).To(Equal(withBearer(token)))

				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})

			It("returns an error when the matching public key cannot be retrieved from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)
				newPrivateKey := generateLegitTokenKey("testKey2")

				payload := tc.BuildValidPayload("logs.admin")
				token := tc.CreateSignedTokenUsingPrivateKey(payload, newPrivateKey)

				_, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))

				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})

			It("returns an error when given a token signed by an public key that was purged from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)

				payload := tc.BuildValidPayload("logs.admin")
				tokenSignedWithExpiredPrivateKey := tc.CreateSignedToken(payload)

				newAndOnlyPrivateKey := generateLegitTokenKey("testKey2")
				tc.MockUAATokenKeyResponseUsingPrivateKey(newAndOnlyPrivateKey)
				payload = tc.BuildValidPayload("logs.admin")
				tokenSignedWithNewPrivateKey := tc.CreateSignedTokenUsingPrivateKey(payload, newAndOnlyPrivateKey)

				_, err := tc.uaaClient.Read(withBearer(tokenSignedWithNewPrivateKey))
				Expect(err).ToNot(HaveOccurred())

				_, err = tc.uaaClient.Read(withBearer(tokenSignedWithExpiredPrivateKey))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))

				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 2))
			})

			It("continues to accept previously signed tokens when retrieving public keys from UAA fails", func() {
				initialRequestCount := len(tc.httpClient.requests)

				payload := tc.BuildValidPayload("logs.admin")
				toBeExpiredToken := tc.CreateSignedToken(payload)

				_, err := tc.uaaClient.Read(withBearer(toBeExpiredToken))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount))

				newTokenKey := generateLegitTokenKey("testKey2")
				refreshedToken := tc.CreateSignedTokenUsingPrivateKey(payload, newTokenKey)
				newTokenKey.publicKey = "corrupted public key"
				tc.MockUAATokenKeyResponseUsingPrivateKey(newTokenKey)

				_, err = tc.uaaClient.Read(withBearer(refreshedToken))
				Expect(err).To(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))

				_, err = tc.uaaClient.Read(withBearer(toBeExpiredToken))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})
		})

		It("returns an error when given a token signed by an unknown but valid key", func() {
			initialRequestCount := len(tc.httpClient.requests)

			unknownPrivateKey := generateLegitTokenKey("testKey99")
			payload := tc.BuildValidPayload("logs.admin")
			tokenSignedWithUnknownPrivateKey := tc.CreateSignedTokenUsingPrivateKey(payload, unknownPrivateKey)

			_, err := tc.uaaClient.Read(withBearer(tokenSignedWithUnknownPrivateKey))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))

			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
		})

		It("returns an error when the provided token cannot be decoded", func() {
			_, err := tc.uaaClient.Read("any-old-token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode token"))
		})

		DescribeTable("handling the Bearer prefix in the Authorization header",
			func(prefix string) {
				payload := tc.BuildValidPayload("foo.bar")
				token := tc.CreateSignedToken(payload)

				c, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Token).To(Equal(withBearer(token)))
			},
			Entry("Standard 'Bearer' prefix", "Bearer "),
			Entry("Non-Standard 'bearer' prefix", "bearer "),
			Entry("No prefix", ""),
		)
	})

	Context("RefreshTokenKeys()", func() {
		It("handles concurrent refreshes", func() {
			tc := uaaSetup(true)
			tc.GenerateSingleTokenKeyResponse(true)

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())

			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)

			numRequests := len(tc.httpClient.requests)

			var wg sync.WaitGroup

			for n := 0; n < 4; n++ {
				wg.Add(1)
				//nolint:errcheck
				go func(wg *sync.WaitGroup) {
					tc.uaaClient.Read(withBearer(token))
					tc.uaaClient.RefreshTokenKeys()
					tc.uaaClient.Read(withBearer(token))
					wg.Done()
				}(&wg)
			}

			wg.Wait()

			Expect(len(tc.httpClient.requests)).To(Equal(numRequests + 4))
		})

		It("calls UAA correctly", func() {
			tc := uaaSetup(true)
			tc.GenerateSingleTokenKeyResponse(true)
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())

			r := tc.httpClient.requests[0]

			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
			Expect(r.URL.Path).To(Equal("/token_keys"))

			// confirm that we're not using any authentication
			_, _, ok := r.BasicAuth()
			Expect(ok).To(BeFalse())

			Expect(r.Body).To(BeNil())
		})

		It("calls UAA with basic auth", func() {
			tc := uaaSetup(true, auth.WithBasicAuth("User", "Password"))
			tc.GenerateSingleTokenKeyResponse(true)
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())

			r := tc.httpClient.requests[0]

			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
			Expect(r.URL.Path).To(Equal("/token_keys"))

			// confirm that we're not using any authentication
			user, password, ok := r.BasicAuth()
			Expect(ok).To(BeTrue())
			Expect(user).To(Equal("User"))
			Expect(password).To(Equal("Password"))
		})

		It("returns an error when UAA cannot be reached", func() {
			tc := uaaSetup(true)
			tc.httpClient.resps = []response{{
				err: errors.New("error!"),
			}}

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when UAA returns a non-200 response", func() {
			tc := uaaSetup(true)
			tc.httpClient.resps = []response{{
				body:   []byte{},
				status: http.StatusUnauthorized,
			}}

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA is malformed", func() {
			tc := uaaSetup(true)
			tc.httpClient.resps = []response{{
				body:   []byte("garbage"),
				status: http.StatusOK,
			}}

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA has an empty key", func() {
			tc := uaaSetup(true)
			tc.GenerateEmptyTokenKeyResponse()

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA has an unparsable PEM format", func() {
			tc := uaaSetup(true)
			tc.GenerateTokenKeyResponseWithInvalidPEM()

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to parse PEM block containing the public key"))
		})

		It("returns an error when the response from the UAA has an invalid key format", func() {
			tc := uaaSetup(true)
			tc.GenerateTokenKeyResponseWithInvalidKey()

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error parsing public key"))
		})

		It("overwrites a pre-existing keyId with the new key", func() {
			tc := uaaSetup(true)
			tc.PrimePublicKeyCache(true)

			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)

			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).NotTo(HaveOccurred())

			tokenKey := generateLegitTokenKey("testKey1")
			tc.GenerateTokenKeyResponse(true, []mockTokenKey{tokenKey})
			err = tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())

			_, err = tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())

			newToken := tc.CreateSignedTokenUsingPrivateKey(payload, tokenKey)
			_, err = tc.uaaClient.Read(withBearer(newToken))
			Expect(err).NotTo(HaveOccurred())
		})

		It("overwrites a pre-existing keyId with the new key", func() {
			tc := uaaSetup(true)
			tc.PrimePublicKeyCache(true)

			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)

			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).NotTo(HaveOccurred())

			tokenKey := generateLegitTokenKey("testKey1")
			tc.GenerateTokenKeyResponse(true, []mockTokenKey{tokenKey})
			newToken := tc.CreateSignedTokenUsingPrivateKey(payload, tokenKey)

			Eventually(func() bool {
				_, err = tc.uaaClient.Read(withBearer(newToken))
				return err == nil
			}).Should(BeTrue())

			_, err = tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("crypto/rsa: verification error"))
		})

		It("rate limits UAA TokenKey refreshes", func() {
			tc := uaaSetup(true, auth.WithMinimumRefreshInterval(200*time.Millisecond))
			tc.GenerateSingleTokenKeyResponse(true)

			initialRequestCount := len(tc.httpClient.requests)

			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))

			time.Sleep(100 * time.Millisecond)
			err = tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))

			time.Sleep(101 * time.Millisecond)
			err = tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 2))
		})
	})
})

type mockTokenKey struct {
	privateKey string
	publicKey  string
	keyId      string
}

func generateLegitTokenKey(keyId string) mockTokenKey {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKeyString, privateKeyString := keyPEMToString(privateKey)

	return mockTokenKey{
		privateKey: privateKeyString,
		publicKey:  publicKeyString,
		keyId:      keyId,
	}
}

// func generateHSTokenKey(keyId string) mockTokenKey {
// 	privateKey := keyId

// 	return mockTokenKey{
// 		privateKey: privateKey,
// 		publicKey:  privateKey,
// 		keyId:      keyId,
// 	}
// }

func uaaSetup(rsa bool, opts ...auth.UAAOption) *UAATestContext {
	httpClient := newSpyHTTPClient()
	metrics := testhelpers.NewMetricsRegistry()
	var tokenKey mockTokenKey
	if rsa {
		tokenKey = generateLegitTokenKey("testKey1")
	} else {
		tokenKey = mockTokenKey{
			privateKey: "key",
			publicKey:  "key",
			keyId:      "key",
		}
	}
	// default the minimumRefreshInterval in tests to 0, but make sure we
	// apply user-provided options afterwards
	opts = append([]auth.UAAOption{auth.WithMinimumRefreshInterval(0)}, opts...)

	uaaClient := auth.NewUAAClient(
		"https://uaa.com",
		httpClient,
		metrics,
		log.New(io.Discard, "", 0),
		opts...,
	)

	return &UAATestContext{
		uaaClient:   uaaClient,
		httpClient:  httpClient,
		metrics:     metrics,
		privateKeys: []mockTokenKey{tokenKey},
	}
}

type UAATestContext struct {
	uaaClient   *auth.UAAClient
	httpClient  *spyHTTPClient
	metrics     *testhelpers.SpyMetricsRegistry
	privateKeys []mockTokenKey
}

func (tc *UAATestContext) PrimePublicKeyCache(rsa bool) {
	tc.GenerateSingleTokenKeyResponse(rsa)

	err := tc.uaaClient.RefreshTokenKeys()
	Expect(err).ToNot(HaveOccurred())
}

func (tc *UAATestContext) BuildValidPayload(scope string) string {
	t := time.Now().Add(time.Hour).Truncate(time.Second)
	payload := fmt.Sprintf(`{"scope":["%s"], "exp":%d}`, scope, t.Unix())

	return payload
}

func (tc *UAATestContext) BuildExpiredPayload(scope string) string {
	t := time.Now().Add(-time.Minute)
	payload := fmt.Sprintf(`{"scope":["%s"], "exp":%d}`, scope, t.Unix())

	return payload
}

func (tc *UAATestContext) GenerateTokenKeyResponse(rsa bool, mockTokenKeys []mockTokenKey) {
	var tokenKeys []map[string]string
	var kty, alg string
	if rsa {
		kty = "RSA"
		alg = "RSA256"
	} else {
		kty = "MAC"
		alg = "HS256"
	}
	for _, mockPrivateKey := range mockTokenKeys {
		tokenKey := map[string]string{
			"kty":   kty,
			"use":   "sig",
			"kid":   mockPrivateKey.keyId,
			"alg":   alg,
			"value": mockPrivateKey.publicKey,
		}
		tokenKeys = append(tokenKeys, tokenKey)

	}
	data, err := json.Marshal(map[string][]map[string]string{
		"keys": tokenKeys,
	})

	Expect(err).ToNot(HaveOccurred())

	tc.httpClient.resps = []response{{
		body:   data,
		status: http.StatusOK,
	}}
}

func (tc *UAATestContext) GenerateSingleTokenKeyResponse(rsa bool) {
	tc.GenerateTokenKeyResponse(
		rsa,
		[]mockTokenKey{
			tc.privateKeys[0],
		},
	)
}

func (tc *UAATestContext) MockUAATokenKeyResponseUsingPrivateKey(tokenKey mockTokenKey) {
	tc.GenerateTokenKeyResponse(
		true,
		[]mockTokenKey{
			tokenKey,
		},
	)
}

func (tc *UAATestContext) AddPrivateKeyToUAATokenKeyResponse(tokenKey mockTokenKey) {
	tc.GenerateTokenKeyResponse(
		true,
		[]mockTokenKey{
			tokenKey,
			tc.privateKeys[0],
		},
	)
}

func (tc *UAATestContext) GenerateEmptyTokenKeyResponse() {
	tc.GenerateTokenKeyResponse(
		true,
		[]mockTokenKey{
			{publicKey: "", keyId: ""},
		},
	)
}

func (tc *UAATestContext) GenerateTokenKeyResponseWithInvalidPEM() {
	tc.GenerateTokenKeyResponse(
		true,
		[]mockTokenKey{
			{publicKey: "-- BEGIN SOMETHING --\nNOTVALIDPEM\n-- END SOMETHING --\n", keyId: ""},
		},
	)
}

func (tc *UAATestContext) GenerateTokenKeyResponseWithInvalidKey() {
	tc.GenerateTokenKeyResponse(
		true,
		[]mockTokenKey{
			{publicKey: strings.Replace(tc.privateKeys[0].publicKey, "MIIB", "XXXX", 1), keyId: ""},
		},
	)
}

func (tc *UAATestContext) CreateSignedToken(payload string) string {
	tokenKey := tc.privateKeys[0]
	decode, _ := pem.Decode([]byte(tokenKey.privateKey))
	privateKey, err := x509.ParsePKCS1PrivateKey(decode.Bytes)
	Expect(err).ToNot(HaveOccurred())
	token, err := jose.Sign(payload, jose.RS256, privateKey, jose.Header("kid", tokenKey.keyId))
	Expect(err).ToNot(HaveOccurred())

	return token
}

func (tc *UAATestContext) CreateHS256SignedToken(payload string) string {
	tokenKey := tc.privateKeys[0]
	token, err := jose.Sign(payload, jose.HS256, []byte(tokenKey.privateKey), jose.Header("kid", tokenKey.keyId))
	Expect(err).ToNot(HaveOccurred())

	return token
}

func (tc *UAATestContext) CreateSignedTokenUsingPrivateKey(payload string, tokenKey mockTokenKey) string {
	decode, _ := pem.Decode([]byte(tokenKey.privateKey))
	privateKey, err := x509.ParsePKCS1PrivateKey(decode.Bytes)
	Expect(err).ToNot(HaveOccurred())
	token, err := jose.Sign(payload, jose.RS256, privateKey, jose.Header("kid", tokenKey.keyId))
	Expect(err).ToNot(HaveOccurred())

	return token
}

func (tc *UAATestContext) CreateUnsignedToken(payload string) string {
	token, err := jose.Sign(payload, jose.NONE, nil)
	Expect(err).ToNot(HaveOccurred())

	return token
}

type spyHTTPClient struct {
	mu       sync.Mutex
	requests []*http.Request
	resps    []response
	tokens   []string
}

type response struct {
	status int
	err    error
	body   []byte
}

func newSpyHTTPClient() *spyHTTPClient {
	return &spyHTTPClient{}
}

func (s *spyHTTPClient) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, r)
	s.tokens = append(s.tokens, r.Header.Get("Authorization"))

	if len(s.resps) == 0 {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}

	result := s.resps[0]
	s.resps = s.resps[1:]

	resp := http.Response{
		StatusCode: result.status,
		Body:       io.NopCloser(bytes.NewReader(result.body)),
	}

	if result.err != nil {
		return nil, result.err
	}

	return &resp, nil
}

func keyPEMToString(privateKey *rsa.PrivateKey) (string, string) {
	encodedKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	Expect(err).ToNot(HaveOccurred())

	var pemKey = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: encodedKey,
	}
	publicKey := string(pem.EncodeToMemory(pemKey))

	encodedKey = x509.MarshalPKCS1PrivateKey(privateKey)

	pemKey = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: encodedKey,
	}
	privateKeyString := string(pem.EncodeToMemory(pemKey))
	return publicKey, privateKeyString
}

func withBearer(token string) string {
	return fmt.Sprintf("Bearer %s", token)
}
