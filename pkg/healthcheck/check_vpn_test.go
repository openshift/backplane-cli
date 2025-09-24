package healthcheck_test

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/healthcheck"
	healthcheckMock "github.com/openshift/backplane-cli/pkg/healthcheck/mocks"
)

var _ = Describe("VPN Connectivity", func() {
	var (
		mockCtrl       *gomock.Controller
		mockInterfaces *healthcheckMock.MockNetworkInterface
		mockClient     *healthcheckMock.MockHTTPClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockInterfaces = healthcheckMock.NewMockNetworkInterface(mockCtrl)
		mockClient = healthcheckMock.NewMockHTTPClient(mockCtrl)
		healthcheck.NetInterfaces = mockInterfaces
		healthcheck.HTTPClients = mockClient
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("CheckVPNConnectivity", func() {
		var originalGetVPNCheckEndpointFunc func() (string, error)

		BeforeEach(func() {
			originalGetVPNCheckEndpointFunc = healthcheck.GetVPNCheckEndpointFunc
		})

		AfterEach(func() {
			healthcheck.GetVPNCheckEndpointFunc = originalGetVPNCheckEndpointFunc
		})

		Context("When checking VPN connectivity", func() {
			It("should pass if VPN is connected on Linux", func() {
				interfaces := []net.Interface{{Name: "tun0"}}
				vpnEndpoint := "http://vpn-endpoint"

				healthcheck.GetVPNCheckEndpointFunc = func() (string, error) {
					return vpnEndpoint, nil
				}

				mockInterfaces.EXPECT().Interfaces().Return(interfaces, nil).AnyTimes()
				mockClient.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				vpnEndpoint = server.URL

				err := healthcheck.CheckVPNConnectivity(mockInterfaces, mockClient)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should pass if VPN is connected on macOS", func() {
				interfaces := []net.Interface{{Name: "utun0"}}
				vpnEndpoint := "http://vpn-endpoint"

				healthcheck.GetVPNCheckEndpointFunc = func() (string, error) {
					return vpnEndpoint, nil
				}

				mockInterfaces.EXPECT().Interfaces().Return(interfaces, nil).AnyTimes()
				mockClient.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				vpnEndpoint = server.URL

				err := healthcheck.CheckVPNConnectivity(mockInterfaces, mockClient)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail if VPN is not connected", func() {
				interfaces := []net.Interface{{Name: "eth0"}}
				vpnEndpoint := "http://vpn-endpoint"

				healthcheck.GetVPNCheckEndpointFunc = func() (string, error) {
					return vpnEndpoint, nil
				}

				mockInterfaces.EXPECT().Interfaces().Return(interfaces, nil).AnyTimes()
				mockClient.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

				err := healthcheck.CheckVPNConnectivity(mockInterfaces, mockClient)
				Expect(err).To(HaveOccurred())
			})

			It("should fail if no VPN interfaces are found", func() {
				interfaces := []net.Interface{}
				vpnEndpoint := "http://vpn-endpoint"

				healthcheck.GetVPNCheckEndpointFunc = func() (string, error) {
					return vpnEndpoint, nil
				}

				mockInterfaces.EXPECT().Interfaces().Return(interfaces, nil).AnyTimes()
				mockClient.EXPECT().Get(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK}, nil).AnyTimes()

				err := healthcheck.CheckVPNConnectivity(mockInterfaces, mockClient)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetVPNCheckEndpoint", func() {
		var originalGetConfigFunc func() (config.BackplaneConfiguration, error)

		BeforeEach(func() {
			originalGetConfigFunc = healthcheck.GetConfigFunc
		})

		AfterEach(func() {
			healthcheck.GetConfigFunc = originalGetConfigFunc
		})

		It("should return the configured VPN endpoint", func() {
			vpnEndpoint := "http://vpn-endpoint"
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{VPNCheckEndpoint: vpnEndpoint}, nil
			}

			endpoint, err := healthcheck.GetVPNCheckEndpoint()
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(vpnEndpoint))
		})

		It("should return an error if VPN endpoint is not configured", func() {
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{VPNCheckEndpoint: ""}, nil
			}

			_, err := healthcheck.GetVPNCheckEndpoint()
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if failed to get backplane configuration", func() {
			healthcheck.GetConfigFunc = func() (config.BackplaneConfiguration, error) {
				return config.BackplaneConfiguration{}, errors.New("failed to get backplane configuration")
			}

			_, err := healthcheck.GetVPNCheckEndpoint()
			Expect(err).To(HaveOccurred())
		})
	})
})
