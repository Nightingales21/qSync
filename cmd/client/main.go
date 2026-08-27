// Command client is the qSync client: it connects to a qSync server, fetches
// its manifest, diffs it against a local directory, and prints the resulting
// sync plan. Actual file transfer is implemented in a later phase.
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
	addr := flag.String("addr", "127.0.0.1:4433", "server address to connect to")
	dir := flag.String("dir", ".", "local directory to sync into")
	withDeletes := flag.Bool("delete", false, "also report files present locally but absent on the server")
	flag.Parse()

	tlsConf := certutil.ClientTLSConfig(alpn)

	conn, err := quic.DialAddr(context.Background(), *addr, tlsConf, nil)
	if err != nil {
		log.Fatalf("dial %s: %v", *addr, err)
	}
	defer conn.CloseWithError(0, "")

	ctrl, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("open control stream: %v", err)
	}
	defer ctrl.Close()

	if err := protocol.WriteMessage(ctrl, protocol.NewManifestRequest()); err != nil {
		log.Fatalf("send manifest request: %v", err)
	}

	resp, err := protocol.ReadMessage(ctrl)
	if err != nil {
		log.Fatalf("read manifest response: %v", err)
	}
	if resp.Type != protocol.MsgManifestResponse || resp.Manifest == nil {
		log.Fatalf("unexpected response type %s", resp.Type)
	}

	log.Printf("received server manifest: %d entries", len(resp.Manifest.Entries))

	local, err := manifest.Scan(*dir)
	if err != nil {
		log.Fatalf("scan local dir %s: %v", *dir, err)
	}

	plan := manifest.Diff(local, *resp.Manifest, *withDeletes)

	if len(plan) == 0 {
		log.Println("already in sync, nothing to do")
		return
	}

	log.Printf("sync plan: %d change(s)", len(plan))
	for _, item := range plan {
		log.Printf("  %-8s %s (%d bytes)", item.Action, item.Path, item.Size)
	}
}
