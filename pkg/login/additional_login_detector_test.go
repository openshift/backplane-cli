package login

import (
	"bytes"
	"crypto/md5" // #nosec
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ctest "k8s.io/client-go/testing"
)

var successfulTokenReviewResponse *authv1.TokenReview = &authv1.TokenReview{
	TypeMeta: metav1.TypeMeta{
		Kind:       "TokenReview",
		APIVersion: "authentication.k8s.io/v1",
	},
	Status: authv1.TokenReviewStatus{
		Authenticated: true,
		Audiences:     []string{"ServiceAudience"},
		User:          authv1.UserInfo{Username: "system:serviceaccount:openshift-backplane-srep:sre000"},
	},
}

var _ = Describe("AdditionalLoginDetector", func() {
	Describe("Validates printing logic", func() {
		It("Prints that there is no users when an empty session map is provided", func() {
			sessions := map[string]int{}
			var output bytes.Buffer

			PrintSessions(&output, sessions)
			Expect(output.String()).Should(ContainSubstring("no other"))
		})
		It("Prints the roles correctly", func() {
			var output bytes.Buffer
			sessions := map[string]int{
				"role_one":    2,
				"anotherRole": 5,
			}

			PrintSessions(&output, sessions)
			Expect(output.String()).Should(ContainSubstring("Checking for other"))
			Expect(output.String()).Should(ContainSubstring("2 other users logged in under the role_one"))
			Expect(output.String()).Should(ContainSubstring("5 other users logged in under the anotherRole"))
		})
	})

	Describe("Validate service account user string splitting", func() {
		It("Validates that an error is returned when a slice is too small", func() {
			_, _, err := splitServiceAccountUserString("system:serviceaccount:myuser")
			Expect(err).Should(HaveOccurred())
		})

		It("Validates that we return the correct namespace/user combo", func() {
			ns, user, err := splitServiceAccountUserString("system:serviceaccount:myns:myuser")

			Expect(err).NotTo(HaveOccurred())
			Expect(ns).To(Equal("myns"))
			Expect(user).To(Equal("myuser"))
		})
	})

	Describe("Validate Username from Error", func() {
		It("gets the username from a valid error message", func() {
			providedErr := fmt.Errorf("Error: User \"system:serviceaccount:namespace:serviceaccountname\" cannot do the thing")
			username, err := getUsernameFromError(providedErr)
			Expect(username).To(Equal("system:serviceaccount:namespace:serviceaccountname"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns an error when the username cannot be parsed", func() {
			providedErr := fmt.Errorf("Some Other Error")
			username, err := getUsernameFromError(providedErr)

			Expect(username).To(Equal(""))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Validate Config Token Getter", func() {
		It("gets a provided bearer token", func() {
			cfg := &rest.Config{}
			cfg.BearerToken = "abcdefg"

			token, err := getTokenFromConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(Equal("abcdefg"))
		})

		It("gets a provided bearer token from a file", func() {
			// Create a temp dir and defer the cleanup of it
			dir, err := os.MkdirTemp("", "testBearerToken")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(dir) }() // clean up

			// create the bearer token file in the tmp dir
			file := filepath.Join(dir, "bearerTokenfile")
			err = os.WriteFile(file, []byte("mytoken"), 0666) //nolint:gosec
			Expect(err).NotTo(HaveOccurred())

			// Test the file reading
			cfg := &rest.Config{}
			cfg.BearerTokenFile = file

			token, err := getTokenFromConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(Equal("mytoken"))
		})

		It("returns an error if the file doesn't exist", func() {
			fileLocation := "/tmp/12345/this_file_shouldnt_exist"

			cfg := &rest.Config{}
			cfg.BearerTokenFile = fileLocation

			token, err := getTokenFromConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(token).To(Equal(""))
		})
	})

	Describe("validate the whoami token review", func() {
		It("validates a good token", func() {
			client := fake.NewSimpleClientset()
			// We have to Prepend the reactor because of https://github.com/kubernetes/client-go/issues/500
			client.PrependReactor("create", "tokenreviews", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
				return true, successfulTokenReviewResponse, nil
			})

			whoami, err := whoami(client, "abcdefg")
			Expect(err).NotTo(HaveOccurred())
			Expect(whoami).To(Equal("system:serviceaccount:openshift-backplane-srep:sre000"))
		})

		Context("Validate error handling", func() {
			It("validates forbidden error", func() {
				client := fake.NewSimpleClientset()
				client.PrependReactor("create", "tokenreviews", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, k8serrors.NewForbidden(schema.GroupResource{Group: "authentication.k8s.io", Resource: "TokenReview"}, "", fmt.Errorf("User \"system:serviceaccount:myns:user\" cannot do the thing"))
				})

				whoami, err := whoami(client, "abcdefg")
				Expect(err).NotTo(HaveOccurred())
				Expect(whoami).To(Equal("system:serviceaccount:myns:user"))
			})

			It("validates non-forbidden error", func() {
				expectedErr := fmt.Errorf("This is an error")
				client := fake.NewSimpleClientset()
				client.PrependReactor("create", "tokenreviews", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, expectedErr
				})

				whoami, err := whoami(client, "abcdefg")
				Expect(whoami).To(Equal(""))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedErr))
			})

			It("validates Status error", func() {
				client := fake.NewSimpleClientset()
				client.PrependReactor("create", "tokenreviews", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
					tokenReview := &authv1.TokenReview{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TokenReview",
							APIVersion: "authentication.k8s.io/v1",
						},
						Status: authv1.TokenReviewStatus{
							Error: "Some Error",
						},
					}
					return true, tokenReview, nil
				})

				whoami, err := whoami(client, "abcdefg")
				Expect(whoami).To(Equal(""))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Errorf("unexpected status error from TokenReview: Some Error")))
			})
		})
	})

	Describe("Validates Finding of other sessions", func() {
		It("successfully returns a set of sessions", func() {
			cfg := &rest.Config{}
			cfg.BearerToken = "abcdefg"

			client := fake.NewSimpleClientset(
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sre000",
						Namespace: "openshift-backplane-srep",
						Labels: map[string]string{
							"managed.openshift.io/backplane": "true",
						},
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcdefghijk",
						Namespace: "openshift-backplane-cee",
						Labels: map[string]string{
							"managed.openshift.io/backplane": "true",
						},
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hijklmnop",
						Namespace: "openshift-backplane-cee",
						Labels: map[string]string{
							"managed.openshift.io/backplane": "true",
						},
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sre111",
						Namespace: "openshift-backplane-srep",
						Labels: map[string]string{
							"managed.openshift.io/backplane": "true",
						},
					},
				},
			)
			client.PrependReactor("create", "tokenreviews", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
				return true, successfulTokenReviewResponse, nil
			})

			sessions, err := FindOtherSessions(client, cfg, "")
			Expect(err).NotTo(HaveOccurred())
			// We populate 2 cee SAs and 2 SRE SAs, so we expect
			// the len of the sessions map to be 2 (cee, srep) and
			// since we "are" sre000, we only expect a value of 1
			// for the srep because we ignore ourself
			Expect(len(sessions)).To(Equal(2))
			Expect(sessions["cee"]).To(Equal(2))
			Expect(sessions["srep"]).To(Equal(1))
		})

		It("bypasses an uknown failure of the whoami function", func() {
			cfg := &rest.Config{}
			cfg.BearerToken = "abcdefg"

			client := fake.NewSimpleClientset(
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%x", md5.Sum([]byte("myUser"))), // #nosec
						Namespace: "openshift-backplane-srep",
						Labels: map[string]string{
							"managed.openshift.io/backplane": "true",
						},
					},
				},
			)

			client.PrependReactor("create", "tokenreviews", func(a ctest.Action) (h bool, r runtime.Object, e error) {
				return true, successfulTokenReviewResponse, fmt.Errorf("UnknownError")
			})

			sessions, err := FindOtherSessions(client, cfg, "myUser")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(sessions)).To(Equal(0))
		})
	})
})
