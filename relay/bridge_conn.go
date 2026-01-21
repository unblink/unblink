package relay

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// BridgeConn implements net.Conn interface for bridge channels
// This allows go2rtc RTSP client to use our bridge data channels directly
// without needing a TCP proxy
type BridgeConn struct {
	bridgeID   string
	nodeConn   *NodeConn
	recvChan   <-chan []byte
	readBuffer []byte
	readMu     sync.Mutex
	closed     bool
	closeMu    sync.Mutex
	closeOnce  sync.Once
}

// NewBridgeConn creates a new bridge connection wrapper
func NewBridgeConn(bridgeID string, nodeConn *NodeConn, recvChan <-chan []byte) *BridgeConn {
	return &BridgeConn{
		bridgeID: bridgeID,
		nodeConn: nodeConn,
		recvChan: recvChan,
	}
}

// Read reads data from the bridge channel into p
func (bc *BridgeConn) Read(p []byte) (n int, err error) {
	bc.readMu.Lock()
	defer bc.readMu.Unlock()

	if bc.closed {
		return 0, io.EOF
	}

	// If we have buffered data, serve from buffer first
	if len(bc.readBuffer) > 0 {
		n = copy(p, bc.readBuffer)
		bc.readBuffer = bc.readBuffer[n:]
		return n, nil
	}

	// Read next chunk from channel
	select {
	case data, ok := <-bc.recvChan:
		if !ok {
			bc.closed = true
			return 0, io.EOF
		}

		// Copy what we can to p
		n = copy(p, data)

		// If we couldn't fit all data, buffer the rest
		if n < len(data) {
			bc.readBuffer = data[n:]
		}

		return n, nil

	case <-time.After(30 * time.Second):
		// Timeout reading from channel
		return 0, errors.New("read timeout")
	}
}

// Write sends data through the bridge to the node
func (bc *BridgeConn) Write(p []byte) (n int, err error) {
	bc.closeMu.Lock()
	defer bc.closeMu.Unlock()

	if bc.closed {
		return 0, io.ErrClosedPipe
	}

	// Make a copy of the data to send
	data := make([]byte, len(p))
	copy(data, p)

	// Send data to node via bridge
	if err := bc.nodeConn.SendData(bc.bridgeID, data); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close closes the bridge connection
func (bc *BridgeConn) Close() error {
	var err error
	bc.closeOnce.Do(func() {
		bc.closeMu.Lock()
		bc.closed = true
		bc.closeMu.Unlock()

		// Note: We don't close the bridge here because it may be managed elsewhere
		// The channel will be closed when the bridge is properly closed via CloseBridge
	})
	return err
}

// LocalAddr returns a dummy local address
func (bc *BridgeConn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	}
}

// RemoteAddr returns a dummy remote address based on the bridge ID
func (bc *BridgeConn) RemoteAddr() net.Addr {
	return &bridgeAddr{bridgeID: bc.bridgeID}
}

// SetDeadline sets the read and write deadlines
func (bc *BridgeConn) SetDeadline(t time.Time) error {
	// Not implemented for channel-based connections
	return nil
}

// SetReadDeadline sets the read deadline
func (bc *BridgeConn) SetReadDeadline(t time.Time) error {
	// Not implemented for channel-based connections
	return nil
}

// SetWriteDeadline sets the write deadline
func (bc *BridgeConn) SetWriteDeadline(t time.Time) error {
	// Not implemented for channel-based connections
	return nil
}

// bridgeAddr implements net.Addr for bridge connections
type bridgeAddr struct {
	bridgeID string
}

func (ba *bridgeAddr) Network() string {
	return "bridge"
}

func (ba *bridgeAddr) String() string {
	return "bridge://" + ba.bridgeID
}
