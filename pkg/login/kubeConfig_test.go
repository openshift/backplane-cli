package login

import (
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Login Kube Config test", func() {

	var (
		testClusterID string
		kubeConfig    api.Config
		kubePath      string
	)

	BeforeEach(func() {

		testClusterID = "test123"
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
		It("should save cluster kube config in cluster folder, and replace it on second call", func() {

			err := SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())

			path, err := CreateClusterKubeConfig(testClusterID, kubeConfig)
			Expect(err).To(BeNil())
			Expect(path).Should(ContainSubstring(testClusterID))

			//check file is exist
			firstStat, err := os.Stat(path)
			Expect(err).To(BeNil())

			time.Sleep(1 * time.Second)
			path, err = CreateClusterKubeConfig(testClusterID, kubeConfig)
			Expect(err).To(BeNil())
			Expect(path).Should(ContainSubstring(testClusterID))

			//check file has been replaced
			secondStat, err := os.Stat(path)
			Expect(err).To(BeNil())
			Expect(firstStat).ToNot(Equal(secondStat))
		})
	})

	Context("Delete kubeconfig ", func() {
		It("should save cluster kube config in cluster folder", func() {

			err := SetKubeConfigBasePath(kubePath)
			Expect(err).To(BeNil())

			path, err := CreateClusterKubeConfig(testClusterID, kubeConfig)
			Expect(err).To(BeNil())

			// check file is exist
			_, err = os.Stat(path)
			Expect(err).To(BeNil())

			// delete kube config
			err = RemoveClusterKubeConfig(testClusterID)
			Expect(err).To(BeNil())

			// check file is not exist
			_, err = os.Stat(path)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
		})
	})
})
