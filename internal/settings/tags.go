package settings

// These bound TagFilter: how many tags it chooses, as many as the library
// keeps, and how long an ID may be, in bytes.
const (
	maxTagFilter = 200
	maxTagID     = 64
)

// cleanTagIDs is ids, the IDs of tags, each once, without those empty,
// too long or past maxTagFilter, in a slice of its own.
func cleanTagIDs(ids []string) []string {
	out := make([]string, 0, min(len(ids), maxTagFilter))
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || len(id) > maxTagID || len(out) == maxTagFilter || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
