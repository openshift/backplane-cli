package ocm_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/ocm/mocks"
)

var _ = Describe("OCM Wrapper Test", func() {
	var (
		ctrl             *gomock.Controller
		mockOcmInterface *mocks.MockOCMInterface
		ocmConnection    *ocmsdk.Connection
	)

	Context("Test OCM Wrapper", func() {

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT()) // Initialize the controller
			mockOcmInterface = mocks.NewMockOCMInterface(ctrl)
			ocmConnection = &ocmsdk.Connection{}
		})

		AfterEach(func() {
			ctrl.Finish() // Ensures that all expectations were met
		})

		It("Should return trusted IPList", func() {
			ip1 := cmv1.NewTrustedIp().ID("100.10.10.10")
			ip2 := cmv1.NewTrustedIp().ID("200.20.20.20")
			expectedIPList, _ := cmv1.NewTrustedIpList().Items(ip1, ip2).Build()
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(expectedIPList, nil).AnyTimes()

			IPList, err := mockOcmInterface.GetTrustedIPList(ocmConnection)
			Expect(err).To(BeNil())
			Expect(len(IPList.Items())).Should(Equal(2))
		})

		It("Should not return errors for empty trusted IPList", func() {
			mockOcmInterface.EXPECT().GetTrustedIPList(gomock.Any()).Return(nil, nil).AnyTimes()
			_, err := mockOcmInterface.GetTrustedIPList(ocmConnection)
			Expect(err).To(BeNil())
		})
	})

	// make sure to have ocm login before running this test
	Context("Test Real OCM Connection", func() {

		var (
			impl     *ocm.DefaultOCMInterfaceImpl
			currtime time.Time
		)

		BeforeEach(func() {
			impl = &ocm.DefaultOCMInterfaceImpl{
				Timeout: 100 * time.Millisecond,
			}
			currtime = time.Now()
		})

		It("Should handle connection reuse, timeout, and deliberate closure safely", func() {

			conn1, _ := impl.SetupOCMConnection()
			time.Sleep(50 * time.Millisecond)

			// reuse
			conn2, _ := impl.SetupOCMConnection()
			conn3, _ := impl.SetupOCMConnection()
			conn4, _ := impl.SetupOCMConnection()

			fmt.Println("conn1, conn2, conn3, conn4 all same status: ",
				conn1 == conn2 && conn2 == conn3 && conn3 == conn4) // should be true

			time.Sleep(200 * time.Millisecond)
			fmt.Println("Waited for:", time.Since(currtime))
			// trigger a new one
			conn, _ := impl.SetupOCMConnection()

			fmt.Println("conn1 and conn same status: ", conn1 == conn) // should be false

			// closed deliberately
			if conn != nil {
				conn.Close() // if someone closes the connection by mistake
			}

			// Routine for closing connection still runs and closes the connection
			// Validate behaviour after a deliberate closure, it shouldn't panic or throw error
			time.Sleep(200 * time.Millisecond)
		})
	})

	// Focus on connection reuse and timeout using a fake client
	Context("Test Fake OCM Client", func() {
		var impl *FakeOCMClient

		BeforeEach(func() {
			impl = &FakeOCMClient{
				timeout: 50 * time.Millisecond,
			}
		})

		It("Should reuse connection before timeout and close after timeout", func() {

			// first connection
			conn1, _ := impl.SetupOCMConnection()
			conn2, _ := impl.SetupOCMConnection()
			Expect(conn1).To(Equal(conn2), "connections should be reused before timeout")
			Expect(conn1.closed).To(BeFalse(), "connection should be active initially")
			Expect(conn1.ocmConnection).ToNot(BeNil(), "real connection should not be nil")

			// wait for timeout to close connection
			time.Sleep(80 * time.Millisecond)

			Expect(conn1.closed).To(BeTrue(), "connection should be closed after timeout")
			Expect(conn1.ocmConnection).To(BeNil(), "internal connection should be nil after close")

			// trigger a new connection
			conn3, _ := impl.SetupOCMConnection()
			fmt.Println("conn1 id:", conn1.id, "conn3 id:", conn3.id)

			Expect(conn3).ToNot(BeNil())
			Expect(conn3.id).ToNot(Equal(conn1.id), "new connection should have a different id")
			Expect(conn3.closed).To(BeFalse(), "new connection should be active")
			Expect(conn3.ocmConnection).ToNot(BeNil(), "real connection should not be nil")

			// wait for new connection to auto-close
			time.Sleep(60 * time.Millisecond)
			Expect(conn3.closed).To(BeTrue(), "new connection should be closed after timeout")
			Expect(conn3.ocmConnection).To(BeNil(), "internal connection should be nil after close")
		})
	})

})

// Fake client
var connCounter int // global counter for unique IDs

type FakeOCMClient struct {
	timeout        time.Duration
	fakeConnection *FakeConnection
	ocmMutex       sync.Mutex
}

type FakeConnection struct {
	id            string
	closed        bool
	ocmConnection *ocmsdk.Connection
}

// SetupOCMConnection simulates creating a connection
func (f *FakeOCMClient) SetupOCMConnection() (*FakeConnection, error) {
	f.ocmMutex.Lock()
	defer f.ocmMutex.Unlock()

	// reuse existing connection if active
	if f.fakeConnection != nil && !f.fakeConnection.closed {
		return f.fakeConnection, nil
	}

	// create new connection
	connCounter++
	conn := &FakeConnection{
		id:            fmt.Sprintf("connection-%d", connCounter),
		closed:        false,
		ocmConnection: &ocmsdk.Connection{},
	}
	f.fakeConnection = conn

	fmt.Println("Creating new OCM connection:", conn.id)

	// start timeout
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	go f.initiateCloseConnection(ctx, cancel, conn)

	return conn, nil
}

// initiateCloseConnection simulates auto-close of connection after timeout
func (f *FakeOCMClient) initiateCloseConnection(ctx context.Context, cancel context.CancelFunc, conn *FakeConnection) {
	if conn != nil {
		fmt.Println("Starting OCM connection timeout:", f.timeout, conn.id)
	}

	<-ctx.Done()
	f.ocmMutex.Lock()
	defer f.ocmMutex.Unlock()

	if conn != nil && !conn.closed {
		fmt.Println("Closing OCM connection after timeout:", f.timeout, conn.id)
		conn.closed = true
		conn.ocmConnection = nil
	}

	cancel()
}
