package login

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

var _ = Describe("PrintClusterInfo", func() {
	var (
		clusterID        string
		buf              *bytes.Buffer
		mockOcmInterface *ocmMock.MockOCMInterface
		mockCtrl         *gomock.Controller
		oldStdout        *os.File
		r, w             *os.File
		ocmConnection    *ocmsdk.Connection
		clusterInfo      *cmv1.Cluster
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocmConnection = nil
		ocm.DefaultOCMInterface = mockOcmInterface

		clusterID = "test-cluster-id"
		buf = new(bytes.Buffer)
		log.SetOutput(buf)

		// Redirect standard output to the buffer
		oldStdout = os.Stdout
		r, w, _ = os.Pipe()
		os.Stdout = w
	})

	AfterEach(func() {
		// Reset the ocm.DefaultOCMInterface to avoid side effects in other tests
		ocm.DefaultOCMInterface = nil
	})

	Context("Cluster protection status", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print cluster information with access protection disabled", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(false, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Disabled\n"))
		})

		It("should print cluster information with access protection enabled", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})
	})

	Context("Limited Support set to 0", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(0)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print if cluster is Fully Supported", func() {

			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()

			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Fully Supported\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})

	})

	Context("Limited Support set to 1", func() {
		BeforeEach(func() {

			clusterInfo, _ = cmv1.NewCluster().
				ID(clusterID).
				Name("Test Cluster").
				CloudProvider(cmv1.NewCloudProvider().ID("aws")).
				State(cmv1.ClusterState("ready")).
				Region(cmv1.NewCloudRegion().ID("us-east-1")).
				Hypershift(cmv1.NewHypershift().Enabled(false)).
				OpenshiftVersion("4.14.8").
				Status(cmv1.NewClusterStatus().LimitedSupportReasonCount(1)).
				Build()

			mockOcmInterface.EXPECT().GetClusterInfoByID(clusterID).Return(clusterInfo, nil).AnyTimes()
			mockOcmInterface.EXPECT().SetupOCMConnection().Return(ocmConnection, nil).AnyTimes()
		})

		It("should print if cluster is Limited Support", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(ocmConnection, clusterID).Return(true, nil).AnyTimes()
			err := PrintClusterInfo(clusterID)
			Expect(err).To(BeNil())

			// Capture the output
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = buf.ReadFrom(r)

			output := buf.String()
			Expect(output).To(ContainSubstring(fmt.Sprintf("Cluster ID:               %s\n", clusterID)))
			Expect(output).To(ContainSubstring("Cluster Name:             Test Cluster\n"))
			Expect(output).To(ContainSubstring("Cluster Status:           ready\n"))
			Expect(output).To(ContainSubstring("Cluster Region:           us-east-1\n"))
			Expect(output).To(ContainSubstring("Cluster Provider:         aws\n"))
			Expect(output).To(ContainSubstring("Hypershift Enabled:       false\n"))
			Expect(output).To(ContainSubstring("Version:                  4.14.8\n"))
			Expect(output).To(ContainSubstring("Limited Support Status:   Limited Support\n"))
			Expect(output).To(ContainSubstring("Access Protection:        Enabled\n"))
		})
	})
})
