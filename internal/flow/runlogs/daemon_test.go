package runlogs

import (
	"net"
	"testing"
	"time"
)

func pipedStream(t *testing.T) (*connStream, net.Conn) {
	t.Helper()
	client, daemon := net.Pipe()
	stream := newConnStream(connStreamParams{Conn: client, Name: "api", WorkDir: "/work/api"})
	t.Cleanup(func() {
		stream.Close()
		daemon.Close()
	})
	return stream, daemon
}

func TestConnStreamDeliversWhatTheJobWrote(t *testing.T) {
	stream, daemon := pipedStream(t)

	go daemon.Write([]byte("\x1b[32mready\x1b[0m"))

	select {
	case chunk := <-stream.Chunks():
		if string(chunk) != "\x1b[32mready\x1b[0m" {
			t.Fatalf("chunk = %q, want the bytes untouched", chunk)
		}
	case <-time.After(time.Second):
		t.Fatal("no chunk delivered")
	}
}

func TestConnStreamWritesToTheJobsStdin(t *testing.T) {
	stream, daemon := pipedStream(t)

	go stream.Write([]byte("q"))

	buf := make([]byte, 1)
	daemon.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := daemon.Read(buf); err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if buf[0] != 'q' {
		t.Fatalf("stdin = %q, want q", buf)
	}
}

func TestConnStreamClosesItsChunksWhenTheOutputEnds(t *testing.T) {
	stream, daemon := pipedStream(t)

	daemon.Close()

	select {
	case _, open := <-stream.Chunks():
		if open {
			t.Fatal("a chunk arrived after the job's output ended")
		}
	case <-time.After(time.Second):
		t.Fatal("the chunks channel stayed open")
	}
}

// A surface that stops reading must not strand the goroutine holding the
// connection: closing releases it even mid-delivery.
func TestConnStreamCloseReleasesTheReader(t *testing.T) {
	stream, daemon := pipedStream(t)

	go func() {
		for i := 0; i < 4; i++ {
			daemon.Write([]byte("line\n"))
		}
	}()

	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	deadline := time.After(time.Second)
	for {
		select {
		case _, open := <-stream.Chunks():
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("the chunks channel stayed open after Close")
		}
	}
}
