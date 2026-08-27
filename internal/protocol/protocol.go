// Package protocol defines the message framing used on qSync's control
// stream (stream 0 of a QUIC connection): manifest exchange and the
// resulting sync plan.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"qsync/internal/manifest"
)

// MessageType identifies the kind of payload carried by a Message.
type MessageType string

const (
	MsgManifestRequest  MessageType = "MANIFEST_REQUEST"
	MsgManifestResponse MessageType = "MANIFEST_RESPONSE"
	MsgSyncPlan         MessageType = "SYNC_PLAN"
	MsgFileComplete     MessageType = "FILE_COMPLETE"
)

// maxMessageSize guards against a malformed/hostile length prefix causing an
// unbounded allocation.
const maxMessageSize = 256 << 20 // 256 MiB

// Message is the envelope written to the control stream. Exactly one of the
// payload fields is populated, matching Type.
type Message struct {
	Type MessageType `json:"type"`

	// ManifestResponse payload.
	Manifest *manifest.Manifest `json:"manifest,omitempty"`

	// SyncPlan payload: the ordered list of files the client should pull,
	// each paired with the QUIC stream ID assigned to carry it.
	Plan []PlanItem `json:"plan,omitempty"`

	// FileComplete payload.
	FileComplete *FileCompleteInfo `json:"file_complete,omitempty"`
}

// PlanItem pairs one diff entry with the stream ID that will carry it.
type PlanItem struct {
	Path     string          `json:"path"`
	Action   manifest.Action `json:"action"`
	Size     int64           `json:"size"`
	StreamID int64           `json:"stream_id"`
}

// FileCompleteInfo acknowledges that a file finished transferring.
type FileCompleteInfo struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// NewManifestRequest builds a MANIFEST_REQUEST message.
func NewManifestRequest() Message {
	return Message{Type: MsgManifestRequest}
}

// NewManifestResponse builds a MANIFEST_RESPONSE message carrying m.
func NewManifestResponse(m manifest.Manifest) Message {
	return Message{Type: MsgManifestResponse, Manifest: &m}
}

// NewSyncPlan builds a SYNC_PLAN message carrying the given plan items.
func NewSyncPlan(items []PlanItem) Message {
	return Message{Type: MsgSyncPlan, Plan: items}
}

// NewFileComplete builds a FILE_COMPLETE message.
func NewFileComplete(path string, success bool, errMsg string) Message {
	return Message{Type: MsgFileComplete, FileComplete: &FileCompleteInfo{
		Path: path, Success: success, Error: errMsg,
	}}
}

// WriteMessage encodes msg as JSON and writes it to w as a 4-byte
// big-endian length prefix followed by the JSON payload.
func WriteMessage(w io.Writer, msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("protocol: marshal message: %w", err)
	}

	var lenPrefix [4]byte
	binary.BigEndian.PutUint32(lenPrefix[:], uint32(len(body)))

	if _, err := w.Write(lenPrefix[:]); err != nil {
		return fmt.Errorf("protocol: write length prefix: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("protocol: write body: %w", err)
	}
	return nil
}

// ReadMessage reads one length-prefixed JSON message from r.
func ReadMessage(r io.Reader) (Message, error) {
	var lenPrefix [4]byte
	if _, err := io.ReadFull(r, lenPrefix[:]); err != nil {
		return Message{}, err
	}

	n := binary.BigEndian.Uint32(lenPrefix[:])
	if n > maxMessageSize {
		return Message{}, fmt.Errorf("protocol: message size %d exceeds max %d", n, maxMessageSize)
	}

	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Message{}, fmt.Errorf("protocol: read body: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return Message{}, fmt.Errorf("protocol: unmarshal message: %w", err)
	}
	return msg, nil
}
