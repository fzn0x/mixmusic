package settings

// These bound CollapsedFolders: how many folders it keeps closed, well
// past what a sidebar holds, and how long an ID may be, in bytes.
const (
	maxCollapsedFolders = 1024
	maxFolderID         = 64
)

// cleanFolderIDs is ids, the IDs of folders, each once, without those
// empty, too long or past maxCollapsedFolders, in a slice of its own.
func cleanFolderIDs(ids []string) []string {
	out := make([]string, 0, min(len(ids), maxCollapsedFolders))
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || len(id) > maxFolderID || len(out) == maxCollapsedFolders || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
