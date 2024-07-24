package healthcheck_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/healthcheck"
	healthcheckMock "github.com/openshift/backplane-cli/pkg/healthcheck/mocks"
)

var _ = Describe("Proxy Connectivity", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *healthcheckMock.MockHTTPClient
		mockProxy  *httptest.Server
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = healthcheckMock.NewMockHTTPClient(mockCtrl)
		healthcheck.HTTPClients = mockClient

		// Set up a mock proxy server
		mockProxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
			proxyURL := mockProxy.URL
			return config.BackplaneConfiguration{ProxyURL: &proxyURL}, nil
		}
	})

	AfterEach(func() {
		mockProxy.Close()
		mockCtrl.Finish()
	})

	Describe("CheckProxyConnectivity", func() {
		var originalGetProxyTestEndpointFunc func() (string, error)

		BeforeEach(func() {
			originalGetProxyTestEndpointFunc = healthcheck.GetProxyTestEndpointFunc
		})

		AfterEach(func() {
			healthcheck.GetProxyTestEndpointFunc = originalGetProxyTestEndpointFunc
		})

		Context("When proxy is not configured", func() {
			It("should return an error", func() {
				healthcheck.GetProxyTestEndpointFunc = func() (string, error) {
					return "", errors.New("proxy test endpoint not configured")
				}

				healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
					return config.BackplaneConfiguration{ProxyURL: nil}, nil
				}

				_, err := healthcheck.CheckProxyConnectivity(mockClient)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When proxy is configured", func() {
			It("should pass if proxy connectivity is good", func() {
				healthcheck.GetProxyTestEndpointFunc = func() (string, error) {
					return "http://proxy-test-endpoint", nil
				}

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()

				healthcheck.GetProxyTestEndpointFunc = func() (string, error) {
					return server.URL, nil
				}

				mockClient.EXPECT().Get(server.URL).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

				url, err := healthcheck.CheckProxyConnectivity(mockClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(Equal(mockProxy.URL))
			})
		})
	})

	Describe("GetProxyTestEndpoint", func() {
		var originalGetConfigFunc func() (config.BackplaneConfiguration, error)

		BeforeEach(func() {
			originalGetConfigFunc = healthcheck.GetConfigFunc
		})

		AfterEach(func() {
			healthcheck.GetConfigFunc = originalGetConfigFunc
		})

		It("should return the configured proxy endpoint", func() {
			proxyEndpoint := "http://proxy-endpoint"
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{ProxyCheckEndpoint: proxyEndpoint}, nil
			}

			endpoint, err := healthcheck.GetProxyTestEndpoint()
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(proxyEndpoint))
		})

		It("should return an error if proxy endpoint is not configured", func() {
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{ProxyCheckEndpoint: ""}, nil
			}

			_, err := healthcheck.GetProxyTestEndpoint()
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if failed to get backplane configuration", func() {
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{}, errors.New("failed to get backplane configuration")
			}

			_, err := healthcheck.GetProxyTestEndpoint()
			Expect(err).To(HaveOccurred())
		})
	})
})
