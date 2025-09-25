package ocm_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

// Establish Ocm Connection before running this test
func TestSetupOCMConnection_Real(t *testing.T) {
	currtime := time.Now()
	impl := &ocm.DefaultOCMInterfaceImpl{
		Timeout: 5 * time.Second,
	}
	conn1, _ := impl.SetupOCMConnection()
	time.Sleep(2 * time.Second)

	// reuse
	conn2, _ := impl.SetupOCMConnection()
	conn3, _ := impl.SetupOCMConnection()
	conn4, _ := impl.SetupOCMConnection()

	fmt.Println("conn1, conn2, conn3, conn4 all same status: ", conn1 == conn2 && conn2 == conn3 && conn3 == conn4) // should be true

	time.Sleep(10 * time.Second)
	fmt.Println("Waited for:", time.Since(currtime))
	//trigger a new one
	conn, _ := impl.SetupOCMConnection()

	// new connection should be created
	fmt.Println("conn1 and conn same status: ", conn1 == conn) // should be false

	//closed deliberately
	if conn != nil {
		conn.Close() // if someone closes the connection by mistake
	}
	// Routine for closing connection still runs and closes the connection
	// Validate behaviour after a deliberate closure, it shouldn't panic or throw error
	time.Sleep(10 * time.Second)
}

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
		ocmConnection: &ocmsdk.Connection{}, // real SDK connection placeholder
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
	fmt.Println("Starting OCM connection timeout:", f.timeout, conn.id)
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

// --- Test ---
func TestSetupOCMConnection_Fake(t *testing.T) {
	impl := &FakeOCMClient{
		timeout: 200 * time.Millisecond, // short timeout for test
	}

	// first connection
	conn1, _ := impl.SetupOCMConnection()
	conn2, _ := impl.SetupOCMConnection()
	assert.Equal(t, conn1, conn2, "connections should be reused before timeout")
	assert.False(t, conn1.closed, "connection should be active initially")
	assert.NotNil(t, conn1.ocmConnection, "real connection should not be nil")

	// wait for timeout to close connection
	time.Sleep(250 * time.Millisecond)

	assert.True(t, conn1.closed, "connection should be closed after timeout")
	assert.Nil(t, conn1.ocmConnection, "internal connection should be nil after close")

	// trigger a new connection
	conn3, _ := impl.SetupOCMConnection()
	fmt.Println("conn1 id:", conn1.id, "conn3 id:", conn3.id)

	assert.NotNil(t, conn3)
	assert.NotEqual(t, conn1.id, conn3.id, "new connection should have a different id")
	assert.False(t, conn3.closed, "new connection should be active")
	assert.NotNil(t, conn3.ocmConnection, "real connection should not be nil")

	// wait for new connection to auto-close
	time.Sleep(250 * time.Millisecond)
	assert.True(t, conn3.closed, "new connection should be closed after timeout")
	assert.Nil(t, conn3.ocmConnection, "internal connection should be nil after close")
}
