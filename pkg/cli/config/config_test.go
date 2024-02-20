package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBackplaneConfig(t *testing.T) {
	t.Run("it returns the user defined proxy instead of the configuration variable", func(t *testing.T) {

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		userDefinedProxy := "example-proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", userDefinedProxy)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		if config.ProxyURL != nil && *config.ProxyURL != userDefinedProxy {
			t.Errorf("expected to return the explicitly defined proxy %v instead of the default one %v", userDefinedProxy, config.ProxyURL)
		}
	})

}

func TestGetBackplaneConnection(t *testing.T) {
	t.Run("should fail if backplane API return connection errors", func(t *testing.T) {

		svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("dummy data"))
		}))

		proxyURL := "http://dummy.proxy"
		t.Setenv("BACKPLANE_URL", svr.URL)
		t.Setenv("HTTPS_PROXY", proxyURL)
		config, err := GetBackplaneConfiguration()
		if err != nil {
			t.Error(err)
		}

		err = config.CheckAPIConnection()
		if err != nil {
			t.Failed()
		}

	})

	t.Run("should fail for empty proxy url", func(t *testing.T) {
		config := BackplaneConfiguration{URL: "https://dummy-url", ProxyURL: nil}
		err := config.CheckAPIConnection()

		if err != nil {
			t.Failed()
		}
	})
}

func TestBackplaneConfiguration_getFirstWorkingProxyURL(t *testing.T) {
	tests := []struct {
		name         string
		proxies      []string
		clientDoFunc func(client *http.Client, req *http.Request) (*http.Response, error)
		want         string
	}{
		{
			name:    "invalid-format-proxy",
			proxies: []string{""},
			want:    "",
		},
		{
			name:    "multiple-invalid-proxies",
			proxies: []string{"-", "gellso", ""},
			want:    "",
		},
		{
			name:    "valid-proxies",
			proxies: []string{"https://dummy.com"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want:    "https://dummy.com",
		},
		{
			name:    "multiple-valid-proxies",
			proxies: []string{"https://dummy.com", "https://dummy.proxy"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want:    "https://dummy.com",
		},
		{
			name:    "multiple-mixed-proxies",
			proxies: []string{"-", "gellso", "https://dummy.com"},
			clientDoFunc: func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK}, nil
			},
			want:    "https://dummy.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("dummy data"))
			}))

			clientDo = tt.clientDoFunc

			config := &BackplaneConfiguration{
				URL: svr.URL,
			}
			got := config.getFirstWorkingProxyURL(tt.proxies)

			if got != tt.want {
				t.Errorf("BackplaneConfiguration.getFirstWorkingProxyURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
