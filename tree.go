package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Node struct {
	Name     string
	Mode     int64
	Hash     string
	Children map[string]*Node
}

// reads objects from staging
func LoadIndex() ([]IndexEntry, error) {
	indexPath := filepath.Join(".gitre", "index")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []IndexEntry{}, nil
		}
		return nil, fmt.Errorf("could not read index: %w", err)
	}

	if len(data) == 0 {
		return []IndexEntry{}, nil
	}

	var entries []IndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("could not parse index JSON: %w", err)
	}

	return entries, nil
}

// builds tree from list of entries
func BuildTree(entries []IndexEntry) *Node {
	root := &Node{
		Name:     "root",
		Mode:     500,
		Children: make(map[string]*Node),
	}

	for _, entry := range entries {
		parts := strings.Split(entry.Path, "/")
		currentNode := root

		for i, part := range parts {
			// check if file or dir
			if i == len(parts)-1 {
				currentNode.Children[part] = &Node{
					Name: part,
					Hash: entry.Hash,
					Mode: entry.Mode,
				}
			} else {
				if _, ok := currentNode.Children[part]; !ok {
					currentNode.Children[part] = &Node{
						Name:     part,
						Mode:     500,
						Children: make(map[string]*Node),
					}
				}
				currentNode = currentNode.Children[part]
			}
		}
	}
	return root
}

func HashStore(data []byte, objType string) (string, error) {
	header := fmt.Sprintf("%s %d\x00", objType, len(data))
	fullContent := append([]byte(header), data...)

	hasher := sha256.New()
	hasher.Write(fullContent)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	dirName := hash[:2]
	fileName := hash[2:]
	objPath := filepath.Join(".gitre", "objects", dirName, fileName)

	if err := os.MkdirAll(filepath.Join(".gitre", "objects", dirName), 0755); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(fullContent); err != nil {
		return "", err
	}
	zw.Close()

	if err := os.WriteFile(objPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return hash, nil
}

// process & compress tree
func WriteTree(node *Node) (string, error) {
	if node.Children == nil {
		return node.Hash, nil
	}

	var treeLines []string
	for name, child := range node.Children {
		childHash, err := WriteTree(child)
		if err != nil {
			return "", err
		}

		itemType := "blob"
		if child.Children != nil {
			itemType = "tree"
		}
		treeLines = append(treeLines, fmt.Sprintf("%d %s %s %s", child.Mode, itemType, childHash, name))
	}
	treeData := []byte(strings.Join(treeLines, "\n"))
	return HashStore(treeData, "tree")
}

// branch tracking
func updateRef(refPath string, hash string) error {
	fullPath := filepath.Join(".gitre", refPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(hash), 0644)
}
