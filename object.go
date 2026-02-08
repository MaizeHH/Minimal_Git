package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type IndexEntry struct {
	Path  string `json:"path"`
	Hash  string `json:"hash"`
	Mode  int64  `json:"mode"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

func IndexObject(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed reading file %s: %w", file, err)
	}
	objectHash, err := HashStore(data, "blob")
	if err != nil {
		return fmt.Errorf("failed to hash file %s: %w", file, err)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("failed to retrieve file information: %w", err)
	}
	entry := IndexEntry{
		Path:  file,
		Hash:  objectHash,
		Mode:  int64(fileInfo.Mode()),
		Size:  fileInfo.Size(),
		Mtime: fileInfo.ModTime().Unix(),
	}

	indexBytes, err := os.ReadFile(filepath.Join(".gitre", "index"))
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var entries []IndexEntry
	if len(indexBytes) == 0 {
		indexBytes = []byte("[]")
	}
	if err := json.Unmarshal(indexBytes, &entries); err != nil {
		return fmt.Errorf("failed to unmarsal index content: %w", err)
	}

	inserted := false
	for i, e := range entries {
		if entry.Path == e.Path {
			entries[i] = entry
			inserted = true
			break
		}
	}
	if !inserted {
		entries = append(entries, entry)
	}

	indexBytes, err = json.MarshalIndent(entries, "", "	")
	if err != nil {
		return fmt.Errorf("failed to marshal json content: %w", err)
	}
	if err = os.WriteFile(filepath.Join(".gitre", "index"), indexBytes, 0600); err != nil {
		return fmt.Errorf("failed to write marshalled content to index: %w", err)
	}

	return nil
}

func HashStore(data []byte, objType string) (string, error) {
	headerString := fmt.Sprintf("%s %d", objType, len(data))

	var content bytes.Buffer
	content.WriteString(headerString)
	content.WriteByte(0)
	content.Write(data)

	var byteContent []byte = content.Bytes()

	hasher := sha256.New()
	hasher.Write(byteContent)
	objectHash := fmt.Sprintf("%x", hasher.Sum(nil))

	dirName := objectHash[:2]
	fileName := objectHash[2:]
	repoDir := filepath.Join(".gitre", "objects")

	if err := os.Mkdir(filepath.Join(repoDir, dirName), 0755); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("failed to create .gitre/objects/%s directory: %w", dirName, err)
	}

	var compressedData bytes.Buffer
	compressWriter := zlib.NewWriter(&compressedData)
	if _, err := compressWriter.Write(byteContent); err != nil {
		return "", fmt.Errorf("failed to compress content: %w", err)
	}
	compressWriter.Close()

	objectPath := filepath.Join(repoDir, dirName, fileName)
	if err := os.WriteFile(objectPath, compressedData.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write object file: %w", err)
	}

	return objectHash, nil
}

func ExtractObject(hash []byte) ([]byte, error) {
	dirName := string(hash[:2])
	fileName := string(hash[2:])
	path := filepath.Join(".gitre", "objects", dirName, fileName)

	compressedData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not find object: %w", err)
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zlib reader: %w", err)
	}
	defer reader.Close()

	var out bytes.Buffer
	_, err = out.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	fullContent := out.Bytes()
	nullIndex := bytes.IndexByte(fullContent, 0)
	if nullIndex == -1 {
		return nil, fmt.Errorf("invalid object format: no null byte found")
	}

	return fullContent[nullIndex+1:], nil
}
