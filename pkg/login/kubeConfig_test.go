package login

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Login Kube Config test", func() {

	var (
		testClusterId string
		kubeConfig    api.Config
		kubePath      string
	)

	BeforeEach(func() {

		testClusterId = "test123"
		kubeConfig = api.Config{
			Kind:        "Config",
			APIVersion:  "v1",
			Preferences: api.Preferences{},
			Clusters: map[string]*api.Cluster{
				"dummy_cluster": {
					Server: "https://api-backplane.apps.something.com/backplane/cluster/configcluster",
				},
			},
		}

		dirName, _ := os.MkdirTemp("", ".kube")
		kubePath = dirName

	})

	Context("save kubeconfig ", func() {
		It("should save cluster kube config in cluster folder", func() {

			err := SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())
			path, err := CreateClusterKubeConfig(testClusterId, kubeConfig)

			Expect(err).To(BeNil())
			Expect(path).Should(ContainSubstring(testClusterId))

			//check file is exist
			_, err = os.Stat(path)
			Expect(err).To(BeNil())
		})
	})

	Context("Delete kubeconfig ", func() {
		It("should save cluster kube config in cluster folder", func() {

			err := SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())

			path, err := CreateClusterKubeConfig(testClusterId, kubeConfig)
			Expect(err).To(BeNil())

			// check file is exist
			_, err = os.Stat(path)
			Expect(err).To(BeNil())

			// delete kube config
			err = RemoveClusterKubeConfig(testClusterId)
			Expect(err).To(BeNil())

			// check file is not exist
			_, err = os.Stat(path)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
		})
	})
})
