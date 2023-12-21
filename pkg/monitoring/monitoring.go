package monitoring

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Masterminds/semver"
	routev1typedclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/pkg/browser"
	logger "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
)

const (
	ALERTMANAGER          = "alertmanager"
	PROMETHEUS            = "prometheus"
	THANOS                = "thanos"
	GRAFANA               = "grafana"
	OpenShiftMonitoringNS = "openshift-monitoring"
)

var (
	MonitoringOpts struct {
		Namespace  string
		Selector   string
		Port       string
		OriginURL  string
		ListenAddr string
		Browser    bool
		KeepAlive  bool
	}
	ValidMonitoringNames = []string{PROMETHEUS, ALERTMANAGER, THANOS, GRAFANA}
)

type Client struct {
	url  string
	http http.Client
}

func NewClient(url string, http http.Client) Client {
	return Client{url, http}
}

// RunMonitoring serve http proxy URL to monitoring dashboard
func (c Client) RunMonitoring(monitoringType string) error {

	// check empty monitoring name
	if monitoringType == "" {
		return fmt.Errorf("monitoring type is empty")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		return err
	}

	// creates URL for serving
	if !strings.Contains(cfg.Host, "backplane/cluster") {
		return fmt.Errorf("the api server is not a backplane url, please make sure you login to the cluster using backplane")
	}

	// set up monitoring Url if it's empty
	if c.url == "" {
		c.url, err = getBackplaneMonitoringURL(monitoringType)
		if err != nil {
			return err
		}
	}

	mURL, err := url.Parse(c.url)
	if err != nil {
		return err
	}
	// creates OCM access token
	accessToken, err := ocm.DefaultOCMInterface.GetOCMAccessToken()
	if err != nil {
		return err
	}

	// check for valid cluster versions
	err = validateClusterVersion(monitoringType)
	if err != nil {
		return err
	}

	isGrafana := monitoringType == GRAFANA
	hasNs := len(MonitoringOpts.Namespace) != 0
	hasAppSelector := len(MonitoringOpts.Selector) != 0
	hasPort := len(MonitoringOpts.Port) != 0
	hasURL := len(MonitoringOpts.OriginURL) != 0

	// serveURL is the port-forward url we print to the user in the end.
	serveURL, err := serveURL(hasURL, hasNs, cfg)
	if err != nil {
		return err
	}

	var name string
	if isGrafana {
		userInterface, err := userv1typedclient.NewForConfig(cfg)
		if err != nil {
			return err
		}

		user, err := userInterface.Users().Get(context.TODO(), "~", metav1.GetOptions{})
		if err != nil {
			return err
		}
		name = strings.Replace(user.Name, "system:serviceaccount:", "", 1)
	}

	// Test if the monitoring stack works, by sending a request to backend/backplane-api
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {

		log.Fatalf("connecting to server %v", err)
	}

	// Add http proxy transport
	proxyURL, err := getProxyURL()
	if err != nil {
		return err
	}
	if proxyURL != nil {
		proxyURL, err := url.Parse(*proxyURL)
		if err != nil {
			return err
		}
		http.DefaultTransport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		logger.Debugf("Using backplane Proxy URL: %s\n", proxyURL)
	}

	req = setProxyRequest(req, mURL, name, accessToken, isGrafana, hasNs, hasAppSelector, hasPort)

	resp, err := c.http.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(responseBody))
	}

	// If the above test pass, we will construct a reverse proxy for the user
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			setProxyRequest(req, mURL, name, accessToken, isGrafana, hasNs, hasAppSelector, hasPort)
		},
	}

	// serve the proxy
	var addr string
	if len(MonitoringOpts.ListenAddr) > 0 {
		// net.Listen will validate the addr later
		addr = MonitoringOpts.ListenAddr
	} else {
		port, err := utils.GetFreePort()
		if err != nil {
			return err
		}
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	serveURL.Host = addr

	if MonitoringOpts.Browser {
		err = browser.OpenURL(serveURL.String())
		if err != nil {
			logger.Warnf("failed opening a browser: %s", err)
		}
	}

	if MonitoringOpts.KeepAlive {
		if !MonitoringOpts.Browser {
			fmt.Printf("Serving %s at %s\n", monitoringType, serveURL.String())
		}
		return http.Serve(l, proxy) //#nosec: G114
	} else {
		return nil
	}

}

// getBackplaneMonitoringURL returns the backplane API monitoring URL based on monitoring type
func getBackplaneMonitoringURL(monitoringType string) (string, error) {
	monitoringURL := ""
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultPathOptions().GetDefaultFilename())
	if err != nil {
		return monitoringURL, err
	}

	// creates URL for serving
	if !strings.Contains(cfg.Host, "backplane/cluster") {
		return monitoringURL, fmt.Errorf("the api server is not a backplane url, please make sure you login to the cluster using backplane")
	}
	monitoringURL = strings.Replace(cfg.Host, "backplane/cluster", fmt.Sprintf("backplane/%s", monitoringType), 1)
	monitoringURL = strings.TrimSuffix(monitoringURL, "/")
	return monitoringURL, nil
}

// Setting Headers, accessToken, port and selector
func setProxyRequest(
	req *http.Request,
	proxyURL *url.URL,
	userName string,
	accessToken *string,
	isGrafana bool,
	hasNs bool,
	hasAppSelector bool,
	hasPort bool,
) *http.Request {
	req.URL.Scheme = "https"
	req.Host = proxyURL.Host
	req.URL.Host = proxyURL.Host
	req.URL.Path = singleJoiningSlash(proxyURL.Path, req.URL.Path)
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}
	if isGrafana {
		req.Header.Set("x-forwarded-user", userName)
	}
	if hasNs {
		req.Header.Set("x-namespace", MonitoringOpts.Namespace)
	}
	if hasAppSelector {
		req.Header.Set("x-selector", MonitoringOpts.Selector)
	}
	if hasPort {
		req.Header.Set("x-port", MonitoringOpts.Port)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *accessToken))

	return req
}

// singleJoiningSlash return format the url slash
func singleJoiningSlash(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	}
	return a + b
}

// Check if a route of the originURL exist in the namespace.
func hasMatchRoute(namespace string, originURL *url.URL, cfg *restclient.Config) bool {
	routeInterface, err := routev1typedclient.NewForConfig(cfg)
	if err != nil {
		logger.Warnf("cannot create route client-go interface %s", err)
		return false
	}
	routes, err := routeInterface.Routes(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Warnf("cannot get routes: %s", err)
		return false
	}
	for _, rt := range routes.Items {
		ri := rt.Status.Ingress
		for _, ig := range ri {
			logger.Debugf("found route ingress %s", ig.Host)
			if utils.MatchBaseDomain(originURL.Hostname(), ig.Host) {
				return true
			}
		}
	}
	return false
}

// serveURL returns the port-forward url to the route
func serveURL(hasURL, hasNs bool, cfg *restclient.Config) (*url.URL, error) {
	serveURL := &url.URL{
		Scheme: "http",
	}

	if hasURL {
		originURL, err := url.Parse(MonitoringOpts.OriginURL)
		if err != nil {
			return nil, err
		}
		// verify if the provided url matches the current login cluster
		currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
		if err != nil {
			return nil, err
		}
		currentCluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(currentClusterInfo.ClusterID)
		if err != nil {
			return nil, err
		}
		baseDomain := currentCluster.DNS().BaseDomain()
		if !utils.MatchBaseDomain(originURL.Hostname(), baseDomain) {
			return nil, fmt.Errorf("the basedomain %s of the current logged cluster %s does not match the provided url, please login to the corresponding cluster first",
				baseDomain, currentClusterInfo.ClusterID)
		}
		// verify if the route exists in the given namespace
		if !hasNs {
			// namespace has a default value, prompt error in case user specify it with blank string.
			return nil, fmt.Errorf("namepace should not be blank, please specify namespace by --namespace")
		}

		if !hasMatchRoute(MonitoringOpts.Namespace, originURL, cfg) {
			return nil, fmt.Errorf("cannot find a matching route in namespace %s for the given url, please specify a correct namespace by --namespace",
				MonitoringOpts.Namespace)
		}

		// append path and query to the url printed later
		serveURL.Path = originURL.Path
		serveURL.RawQuery = originURL.RawQuery
		serveURL.Fragment = originURL.Fragment
		return serveURL, nil
	}

	return serveURL, nil
}

// getProxyURL returns the proxy url
func getProxyURL() (proxyURL *string, err error) {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, err
	}

	return bpConfig.ProxyURL, nil
}

// validateClusterVersion checks the clusterversion based on namespace
func validateClusterVersion(monitoringName string) error {
	if MonitoringOpts.Namespace == OpenShiftMonitoringNS {
		//checks cluster version
		currentClusterInfo, err := utils.DefaultClusterUtils.GetBackplaneClusterFromConfig()
		if err != nil {
			return err
		}

		currentCluster, err := ocm.DefaultOCMInterface.GetClusterInfoByID(currentClusterInfo.ClusterID)
		if err != nil {
			return err
		}

		clusterVersion := currentCluster.OpenshiftVersion()
		if clusterVersion != "" {
			version, err := semver.NewVersion(clusterVersion)
			if err != nil {
				return err
			}
			if version.Minor() >= 11 && (monitoringName == PROMETHEUS || monitoringName == ALERTMANAGER || monitoringName == GRAFANA) {
				return fmt.Errorf("this cluster's version is 4.11 or greater. " +
					"Following version 4.11, Prometheus, AlertManager and Grafana monitoring UIs are deprecated, " +
					"please use 'ocm backplane console' and use the observe tab for the same",
				)
			}
		}
	}
	return nil
}
