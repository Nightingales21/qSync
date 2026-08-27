// Command server is the qSync server: it listens for QUIC connections,
// serves its directory's manifest over the control stream, and (in later
// phases) serves file contents over per-file streams.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/quic-go/quic-go"

	"qsync/internal/certutil"
	"qsync/internal/manifest"
	"qsync/internal/protocol"
)

const alpn = "qsync"

func main() {
	addr := flag.String("addr", "0.0.0.0:4433", "address to listen on")
	dir := flag.String("dir", ".", "directory to serve")
	maxStreams := flag.Int64("max-streams", 100, "max concurrent bidirectional streams accepted from a client")
	flag.Parse()

	tlsConf, err := certutil.GenerateTLSConfig(alpn)
	if err != nil {
		log.Fatalf("generate TLS config: %v", err)
	}

	quicConf := &quic.Config{MaxIncomingStreams: *maxStreams}

	listener, err := quic.ListenAddr(*addr, tlsConf, quicConf)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	log.Printf("qsync server listening on %s, serving %s", *addr, *dir)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handleConn(conn, *dir)
	}
}

func handleConn(conn *quic.Conn, dir string) {
	log.Printf("connection from %s", conn.RemoteAddr())

	ctrl, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Printf("accept control stream: %v", err)
		return
	}
	defer ctrl.Close()

	req, err := protocol.ReadMessage(ctrl)
	if err != nil {
		log.Printf("read manifest request: %v", err)
		return
	}
	if req.Type != protocol.MsgManifestRequest {
		log.Printf("unexpected message type %s on control stream", req.Type)
		return
	}

	m, err := manifest.Scan(dir)
	if err != nil {
		log.Printf("scan %s: %v", dir, err)
		return
	}

	if err := protocol.WriteMessage(ctrl, protocol.NewManifestResponse(m)); err != nil {
		log.Printf("write manifest response: %v", err)
		return
	}

	log.Printf("sent manifest with %d entries to %s", len(m.Entries), conn.RemoteAddr())
}
