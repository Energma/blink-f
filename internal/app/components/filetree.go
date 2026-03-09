package components

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TreeNode represents a directory in the file tree browser.
type TreeNode struct {
	Name      string
	Path      string // absolute path
	IsGitRepo bool   // .git exists in this directory
	Expanded  bool
	Children  []TreeNode
	Loaded    bool // whether children have been fetched
	Depth     int
}

// NewTreeRoot creates a root node for the file tree.
func NewTreeRoot(path string) *TreeNode {
	return &TreeNode{
		Name:  filepath.Base(path),
		Path:  path,
		Depth: 0,
	}
}

// FlattenVisible returns a flat slice of all visible nodes for cursor
// navigation and rendering. Expanded directories show their children;
// collapsed directories hide them.
func FlattenVisible(root *TreeNode) []*TreeNode {
	var result []*TreeNode
	flattenNode(root, &result)
	return result
}

func flattenNode(node *TreeNode, result *[]*TreeNode) {
	*result = append(*result, node)
	if node.Expanded && node.Loaded {
		for i := range node.Children {
			flattenNode(&node.Children[i], result)
		}
	}
}

// FindNode locates a node by its absolute path via DFS.
func FindNode(root *TreeNode, path string) *TreeNode {
	if root.Path == path {
		return root
	}
	for i := range root.Children {
		if n := FindNode(&root.Children[i], path); n != nil {
			return n
		}
	}
	return nil
}

// FindParent returns the parent node of the node at the given path.
func FindParent(root *TreeNode, path string) *TreeNode {
	for i := range root.Children {
		if root.Children[i].Path == path {
			return root
		}
		if p := FindParent(&root.Children[i], path); p != nil {
			return p
		}
	}
	return nil
}

// ReadDirEntries reads a directory and returns TreeNode children.
// Only directories are included (files are skipped). Hidden directories
// (starting with '.') are skipped. Git repos are sorted first.
func ReadDirEntries(parentPath string, depth int) ([]TreeNode, error) {
	entries, err := os.ReadDir(parentPath)
	if err != nil {
		return nil, err
	}

	var nodes []TreeNode
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		fullPath := filepath.Join(parentPath, name)
		isRepo := isGitRepo(fullPath)
		nodes = append(nodes, TreeNode{
			Name:      name,
			Path:      fullPath,
			IsGitRepo: isRepo,
			Depth:     depth,
		})
	}

	// Sort: git repos first, then alphabetical
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsGitRepo != nodes[j].IsGitRepo {
			return nodes[i].IsGitRepo
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// FilterFlatNodes returns indices of flat nodes whose names contain the query (case-insensitive).
func FilterFlatNodes(nodes []*TreeNode, query string) []int {
	if query == "" {
		return nil
	}
	query = strings.ToLower(query)
	var indices []int
	for i, n := range nodes {
		if strings.Contains(strings.ToLower(n.Name), query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
