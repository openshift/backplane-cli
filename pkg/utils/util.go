package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strings"

	netUrl "net/url"

	BackplaneApi "github.com/openshift/backplane-api/pkg/client"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	ClustersPageSize        = 50
	ClusterIDRegexp  string = "/?backplane/cluster/([a-zA-Z0-9]+)/?"
)

// GetFreePort asks the OS for an available port to listen to.
// https://github.com/phayes/freeport/blob/master/freeport.go
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// CheckHealth check if the given url returns http status 200
// return false if it not 200 or encounter any error.
func CheckHealth(url string) bool {
	// Parse the given URL and check for ambiguities
	parsedUrl, err := netUrl.Parse(url)
	if err != nil {
		return false //just return false for any error
	}

	resp, err := http.Get(parsedUrl.String())
	if err != nil {
		return false //just return false for any error
	}
	return resp.StatusCode == http.StatusOK
}

func ReadKubeconfigRaw() (api.Config, error) {
	return genericclioptions.NewConfigFlags(true).ToRawKubeConfigLoader().RawConfig()
}

// MatchBaseDomain returns true if the given longHostname matches the baseDomain.
func MatchBaseDomain(longHostname, baseDomain string) bool {
	if len(baseDomain) == 0 {
		return true
	}
	hostnameSegs := strings.Split(longHostname, ".")
	baseSegs := strings.Split(baseDomain, ".")
	if len(hostnameSegs) < len(baseSegs) {
		return false
	}
	cmpSegs := hostnameSegs[len(hostnameSegs)-len(baseSegs):]

	return reflect.DeepEqual(cmpSegs, baseSegs)
}

func TryParseBackplaneAPIError(rsp *http.Response) (*BackplaneApi.Error, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	var dest BackplaneApi.Error

	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}

	return &dest, nil
}

func TryRenderErrorRaw(rsp *http.Response) error {
	data, err := TryParseBackplaneAPIError(rsp)
	if err != nil {
		return err
	}
	return RenderJsonBytes(data)
}

func GetFormattedError(rsp *http.Response) error {
	data, err := TryParseBackplaneAPIError(rsp)
	if err != nil {
		return err
	}
	return fmt.Errorf("error from backplane: \n Status Code: %d\n Message: %s\n", rsp.StatusCode, *data.Message)
}

func TryPrintAPIError(rsp *http.Response, rawFlag bool) error {
	if rawFlag {
		if err := TryRenderErrorRaw(rsp); err != nil {
			return fmt.Errorf("unable to parse error from backplane: \n Status Code: %d\n", rsp.StatusCode)
		} else {
			return nil
		}
	} else {
		return GetFormattedError(rsp)
	}
}

func ParseParamsFlag(paramsFlag []string) (map[string]string, error) {
	var result = map[string]string{}
	for _, s := range paramsFlag {
		keyVal := strings.Split(s, "=")
		if len(keyVal) >= 2 {
			key := strings.TrimSpace(keyVal[0])
			value := strings.TrimSpace(strings.Join(keyVal[1:], ""))
			result[key] = value
		} else {
			return nil, fmt.Errorf("error parsing params flag, %s", s)
		}
	}
	return result, nil
}
