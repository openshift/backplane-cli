package login

// Functions in this file are built to detect additional backplane logins on the same cluster, in order to inform
// the user who is logging in if other roles may be currently logged in or have logged in recently.

import (
	"context"
	"crypto/md5" // #nosec
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"

	authenticationapi "k8s.io/api/authentication/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	BackplaneUserNamespacePrefix = "openshift-backplane-"
	CEE                          = "cee"
	McsTierTwo                   = "mcs-tier-two"
	LPSRE                        = "lpsre"
	SREP                         = "srep"

	unknownNamespace = "UNKNOWNNAMESPACE"
)

var backplaneUserNamespacesToCheck []string = []string{
	CEE,
	McsTierTwo,
	LPSRE,
	SREP,
}

// FindOtherSessions discovers other active backplane sessions in the cluster.
// It examines service accounts across different backplane user namespaces to identify concurrent sessions.
// Returns a map of role names to session counts, excluding the current user's session.
func FindOtherSessions(clientset kubernetes.Interface, config *rest.Config, user string) (map[string]int, error) {
	sessions := map[string]int{}

	token, err := getTokenFromConfig(config)
	if err != nil {
		logger.Warn("Unable to get token for self review to find other sessions")
		return sessions, err
	}

	myUsername, err := whoami(clientset, token)
	if err != nil {
		logger.WithField("error", err).Debug("Unable to determine who I am to find other sessions")
		// If we cannot determine who we are from the bearer token, fall back
		// to a naive lookup of who we are by transforming the username to the SA name
		saName := userToMd5(user)
		myUsername = fmt.Sprintf("system:serviceaccount:%s:%s", unknownNamespace, saName)
	}

	myNS, myUser, err := splitServiceAccountUserString(myUsername)
	if err != nil {
		return sessions, err
	}

	for _, role := range backplaneUserNamespacesToCheck {
		ns := BackplaneUserNamespacePrefix + role
		saList, err := clientset.CoreV1().ServiceAccounts(ns).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "managed.openshift.io/backplane=true",
		})
		if err != nil {
			logger.WithField("error", err).Debugf("Unable to list %s ServiceAccounts", role)
		}

		count := len(saList.Items)

		logger.Debugf("Found %d service accounts for %s", count, role)

		if count == 0 {
			logger.Debugf("ns is empty, skipping")
			continue
		}

		// Remove me from the count if I'm already logged in
		if ns == myNS || myNS == unknownNamespace {
			logger.Debugf("Checking for my user %q in namespace %q", myUser, ns)
			found := false
			for _, sa := range saList.Items {
				if sa.Name == myUser {
					found = true
					break
				}
			}
			if found {
				count = count - 1
			}
		}

		if count > 0 {
			sessions[role] = count
		}
	}

	return sessions, nil
}

func PrintSessions(w io.Writer, sessions map[string]int) {
	if len(sessions) == 0 {
		fmt.Fprintf(w, "There are no other backplane users logged in.\n")
		return
	}

	fmt.Fprintf(w, "Checking for other backplane sessions:\n")
	for sessionRole, sessionCount := range sessions {
		fmt.Fprintf(w, "  - There are %d other users logged in under the %s role.\n", sessionCount, sessionRole)
	}
}

// Given a standard user account in a format similar to `system:serviceaccount:namespace:username`,
// return the namespace and username and any potential error
func splitServiceAccountUserString(user string) (string, string, error) {
	userSlice := strings.Split(user, ":")
	// meSlice should always be something like
	// system, serviceaccount, openshift-backplane-role-name, username
	if len(userSlice) < 4 {
		return "", "", fmt.Errorf("error splitting user string. Could not parse %s", user)
	}
	myNS := userSlice[2]
	myUser := userSlice[3]

	logger.Debugf("I am %s:%s", myNS, myUser)

	return myNS, myUser, nil
}

// Get the username of the current context from a valid authentication token
func whoami(kubeclient kubernetes.Interface, token string) (string, error) {
	result, err := kubeclient.AuthenticationV1().TokenReviews().Create(context.TODO(), &authenticationapi.TokenReview{
		Spec: authenticationapi.TokenReviewSpec{
			Token: token,
		},
	}, metav1.CreateOptions{})

	if err != nil {
		if k8serrors.IsForbidden(err) {
			return getUsernameFromError(err)
		}
		return "", err
	}

	if result.Status.Error != "" {
		return "", fmt.Errorf("unexpected status error from TokenReview: %v", result.Status.Error)
	}

	return result.Status.User.Username, nil
}

// Given a valid REST Config object, extract the token
func getTokenFromConfig(config *rest.Config) (string, error) {
	c, err := config.TransportConfig()
	if err != nil {
		return "", err
	}

	if c.HasTokenAuth() {
		if config.BearerTokenFile != "" {
			d, err := os.ReadFile(config.BearerTokenFile)
			if err != nil {
				return "", err
			}
			return string(d), nil
		}

		if config.BearerToken != "" {
			return config.BearerToken, nil
		}
	}
	return "", nil
}

// if we get a permissions error, it will tell us the username as part of the error string
// so we can grab the username from that error string
func getUsernameFromError(err error) (string, error) {
	re := regexp.MustCompile(`^.* User "(.*)" cannot .*$`)
	user := re.ReplaceAllString(err.Error(), "$1")
	// if the user string after replacement equals the same string as the whole error, nothing
	// was replaced and we should return an error here.
	if user == err.Error() {
		return "", fmt.Errorf("could not extract username from error string: %v", err)
	}
	return user, nil
}

func userToMd5(user string) string {
	bytes := []byte(user)
	return fmt.Sprintf("%x", md5.Sum(bytes)) // #nosec
}
