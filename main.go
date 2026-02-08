package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no args. available commands: init, add, commit")
		return
	}

	var err error

	switch os.Args[1] {
	case "init":
		if err = initRepo(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "add":
		add(os.Args[2:])
		return
	case "commit":
		commit(os.Args[2])
		return
	case "log":
		log()
		return
	case "status":
		status()
		return
	default:
		fmt.Printf("unknown command: %s. available commands: init, add, commit, status, log\n", os.Args[1])
		return
	}

}

func initRepo() error {
	var repoDir string = ".gitre"
	var dirPerm os.FileMode = 0700
	var filePerm os.FileMode = 0600
	var err error
	if err = os.Mkdir(repoDir, dirPerm); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("failed to create .gitre directory: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(repoDir, "refs", "heads"), dirPerm); err != nil {
		return fmt.Errorf("failed to create refs/heads directory: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(repoDir, "objects"), dirPerm); err != nil {
		return fmt.Errorf("failed to create objects directory: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(repoDir, "refs", "tags"), dirPerm); err != nil {
		return fmt.Errorf("failed to create refs/tags directory: %w", err)
	}

	if _, err = os.Stat(filepath.Join(repoDir, "config")); os.IsNotExist(err) {
		if _, err = os.Create(filepath.Join(repoDir, "config")); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	if _, err = os.Stat(filepath.Join(repoDir, "index")); os.IsNotExist(err) {
		if err = os.WriteFile(filepath.Join(repoDir, "index"), []byte("[]"), filePerm); err != nil {
			return fmt.Errorf("failed to create index file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check index file: %w", err)
	}

	if _, err = os.Stat(filepath.Join(repoDir, "HEAD")); os.IsNotExist(err) {
		if err = os.WriteFile(filepath.Join(repoDir, "HEAD"), []byte("ref: refs/heads/main\n"), filePerm); err != nil {
			return fmt.Errorf("failed to create HEAD file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check HEAD file: %w", err)
	}

	var ignoreContent string = "*.exe\n*.dll\n.env\n"

	if _, err = os.Stat(".gitreignore"); os.IsNotExist(err) {
		if err = os.WriteFile(".gitreignore", []byte(ignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .gitreignore file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check .gitreignore file: %w", err)
	}

	fmt.Println("Initialized gitre repository:", repoDir)

	return nil
}

func add(args []string) []error {
	var err error
	var errors []error
	for _, arg := range args {
		err = IndexObject(arg)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to add %s: %w", arg, err))
		} else {
			fmt.Printf("added %s\n", arg)
		}
	}
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		return errors
	}
	return nil
}

func commit(message string) error {
	entries, err := LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("nothing to commit (index is empty)")
	}

	rootNode := BuildTree(entries)

	rootTreeHash, err := WriteTree(rootNode)
	if err != nil {
		return fmt.Errorf("failed to write tree objects: %w", err)
	}

	head, err := os.ReadFile(".gitre/HEAD")
	ref := bytes.TrimPrefix(head, []byte("ref: "))
	path := filepath.Join(".gitre", string(ref))
	p_hash, err := os.ReadFile(path)
	var commitContent strings.Builder
	commitContent.WriteString(fmt.Sprintf("tree %s\n", rootTreeHash))
	if len(p_hash) > 0 {
		commitContent.WriteString(fmt.Sprintf("parent %s\n", string(p_hash)))
	}
	commitContent.WriteString(message)

	commitHash, err := HashStore([]byte(commitContent.String()), "commit")
	if err != nil {
		return fmt.Errorf("failed to create commit object: %w", err)
	}

	err = updateRef("refs/heads/main", commitHash)
	if err != nil {
		return fmt.Errorf("failed to update ref: %w", err)
	}

	fmt.Printf("[%s] %s\n", commitHash[:7], message)
	return nil
}

func log() error {
	head, err := os.ReadFile(".gitre/HEAD")
	if err != nil {
		return fmt.Errorf("failed to read HEAD: %w", err)
	}
	head = bytes.TrimSpace(head)
	ref := bytes.TrimPrefix(head, []byte("ref: "))
	path := filepath.Join(".gitre", string(ref))
	hash, err := os.ReadFile(path)
	for len(hash) > 0 {
		content, err := ExtractObject(hash)
		if err != nil {
			return fmt.Errorf("error extracting object %s: %w", hash, err)
		}
		fmt.Printf("commit %s\n", hash)
		fmt.Printf("%s", string(content))
		var parentHash []byte
		lines := bytes.SplitSeq(content, []byte("\n"))
		for line := range lines {
			if after, ok := bytes.CutPrefix(line, []byte("parent ")); ok {
				parentHash = after
				break
			}
		}
		hash = parentHash
		if len(hash) > 0 {
			fmt.Println("  |")
		}
	}
	return nil
}

func status() error {
	return nil
}
