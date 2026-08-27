package manifest

// Action classifies how a path differs between the client's local manifest
// and the server's manifest.
type Action string

const (
	// ActionAdd means the file exists on the server but not on the client.
	ActionAdd Action = "add"
	// ActionUpdate means the file exists on both sides but the content hash differs.
	ActionUpdate Action = "update"
	// ActionDelete means the file exists on the client but not on the server.
	// Only produced when the delete flag is enabled.
	ActionDelete Action = "delete"
)

// DiffEntry is one item in the transfer/deletion plan produced by Diff.
type DiffEntry struct {
	Path   string `json:"path"`
	Action Action `json:"action"`
	// Size is the expected size of the file after sync (0 for deletes).
	Size int64 `json:"size"`
}

// Diff compares the client's local manifest against the server's manifest
// and returns the set of changes needed to bring the client up to date with
// the server. withDeletes additionally reports files present on the client
// but absent from the server as ActionDelete entries.
func Diff(client, server Manifest, withDeletes bool) []DiffEntry {
	clientByPath := client.ByPath()
	serverByPath := server.ByPath()

	var plan []DiffEntry

	for _, se := range server.Entries {
		ce, ok := clientByPath[se.Path]
		switch {
		case !ok:
			plan = append(plan, DiffEntry{Path: se.Path, Action: ActionAdd, Size: se.Size})
		case ce.Hash != se.Hash:
			plan = append(plan, DiffEntry{Path: se.Path, Action: ActionUpdate, Size: se.Size})
		}
	}

	if withDeletes {
		for _, ce := range client.Entries {
			if _, ok := serverByPath[ce.Path]; !ok {
				plan = append(plan, DiffEntry{Path: ce.Path, Action: ActionDelete})
			}
		}
	}

	return plan
}
