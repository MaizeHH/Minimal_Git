package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

func HashFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed reading file %s: %w", filePath, err)
	}
	headerString := fmt.Sprintf("blob %d", len(data))
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
	if err = os.Mkdir(filepath.Join(repoDir, dirName), 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("failed to create .gitre/objects/%s directory: %w", dirName, err)
	}
	var compressedData bytes.Buffer
	compressWriter := zlib.NewWriter(&compressedData)
	if _, err = compressWriter.Write(byteContent); err != nil {
		return "", fmt.Errorf("failed to compress content: %w", err)
	}
	compressWriter.Close()
	filePath = filepath.Join(repoDir, dirName, fileName)
	if err = os.WriteFile(filePath, compressedData.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write object file: %w", err)
	}

	return objectHash, nil
}
