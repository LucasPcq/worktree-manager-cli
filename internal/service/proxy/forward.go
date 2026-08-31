package proxy

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ForwardParams struct {
	Listeners []net.Listener
	// Target is the port the run proxy bound. The forwarder speaks no HTTP: the
	// Host header it would need to read is the proxy's business, and a byte
	// relay keeps websockets and streaming working for free.
	Target int
}

// Forward relays every connection the privileged listeners accept to the run
// proxy, and blocks until they all stop accepting.
func Forward(params ForwardParams) error {
	if len(params.Listeners) == 0 {
		return domain.ErrProxyNoListeners
	}

	var wg sync.WaitGroup
	for _, listener := range params.Listeners {
		wg.Add(1)
		go func(l net.Listener) {
			defer wg.Done()
			acceptLoop(l, params.Target)
		}(listener)
	}
	wg.Wait()
	return nil
}

func acceptLoop(listener net.Listener, target int) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go relay(conn, target)
	}
}

func relay(from net.Conn, target int) {
	defer from.Close()

	to, err := net.Dial("tcp", fmt.Sprintf(domain.ProxyLoopbackFmt, target))
	if err != nil {
		return
	}
	defer to.Close()

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(to, from)
		closeWrite(to)
		close(done)
	}()
	_, _ = io.Copy(from, to)
	closeWrite(from)
	<-done
}

// closeWrite half-closes so the peer sees EOF on a response that ends with the
// connection, instead of hanging until a timeout.
func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}
