package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func accumIgnores() []string {
	ignores := []string{".gitre", ".git"}
	data, err := os.ReadFile(".gitreignore")

	if err != nil {
		return ignores
	}
	fileLines := strings.SplitSeq(string(data), "\n")
	for line := range fileLines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			ignores = append(ignores, line)
		}
	}

	return ignores
}

func traverseDir(dir string, ignores []string) ([]string, error) {
	var list []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return list, fmt.Errorf("failed to read dir: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		skip := false
		for _, ignore := range ignores {
			match, _ := filepath.Match(ignore, name)
			if match {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		rawPath := filepath.Join(dir, name)

		if file.IsDir() {
			subFiles, _ := traverseDir(rawPath, ignores)
			list = append(list, subFiles...)
		} else {
			cleanPath := filepath.ToSlash(rawPath)
			cleanPath = strings.TrimPrefix(cleanPath, "./")
			list = append(list, cleanPath)
		}
	}

	return list, nil
}
