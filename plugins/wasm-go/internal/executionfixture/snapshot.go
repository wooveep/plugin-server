// Package executionfixture is an offline D0 test seam. Product plugins must not import it.
package executionfixture

// Snapshot is a private copy of synthetic send inputs, never reconstructed from streamInfo.
type Snapshot struct {
	ID, Call, Attempt   string
	Context, Generation uint32
	Selection           map[string]string
	Body, Outbound      []byte
	Metadata            map[string][]string
}

// CaptureSnapshot retains no mutable input references.
func CaptureSnapshot(source Snapshot) Snapshot {
	got := source
	got.Selection = make(map[string]string, len(source.Selection))
	for key, value := range source.Selection {
		got.Selection[key] = value
	}
	got.Body = append([]byte(nil), source.Body...)
	got.Outbound = append([]byte(nil), source.Outbound...)
	got.Metadata = make(map[string][]string, len(source.Metadata))
	for key, value := range source.Metadata {
		got.Metadata[key] = append([]string(nil), value...)
	}
	return got
}
