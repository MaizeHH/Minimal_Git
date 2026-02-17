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
		fmt.Println("no valid args.")
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
		errs := add(os.Args[2:])
		if len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "Errors occurred while adding files:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  - %v\n", e)
			}
			os.Exit(1)
		}
		return
	case "commit":
		if err = commit(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "log":
		if err = log(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "checkout":
		if err = checkout(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "switch":
		switchBranch()
		return
	case "restore":
		restore()
		return
	case "status":
		if err = status(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	default:
		fmt.Printf("unknown command: %s. available commands: init, add, commit, status, log\n", os.Args[1])
		return
	}

}

func initRepo() error {
	var repoDir string = ".gitre"
	var dirPerm os.FileMode = 0700
	var filePerm os.FileMode = 0644
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
	var errors []error
	files := make(map[string]struct{})
	ignores := accumIgnores()
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			errors = append(errors, fmt.Errorf("path not found: %s", arg))
			continue
		}
		if info.IsDir() {
			diskFiles, _ := traverseDir(arg, ignores)
			for _, f := range diskFiles {
				files[f] = struct{}{}
			}
		} else {
			files[arg] = struct{}{}
		}
	}
	for filePath := range files {
		err := IndexObject(filePath)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to add %s: %w", filePath, err))
		} else {
			fmt.Printf("added %s\n", filePath)
		}
	}
	if len(errors) > 0 {
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
		return fmt.Errorf("nothing to commit")
	}

	rootNode := BuildTree(entries)

	rootTreeHash, err := WriteTree(rootNode)
	if err != nil {
		return fmt.Errorf("failed to write tree objects: %w", err)
	}

	head, err := os.ReadFile(".gitre/HEAD")
	ref := bytes.TrimPrefix(head, []byte("ref: "))
	ref = bytes.TrimSpace(ref)
	path := filepath.Join(".gitre", string(ref))
	p_hash, err := os.ReadFile(path)
	p_hash = bytes.TrimSpace(p_hash)
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

	err = UpdateRef("refs/heads/main", commitHash)
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
		if len(parentHash) > 0 {
			fmt.Println("  |")
		}
		hash = parentHash
	}
	return nil
}

func checkout(name string) error {
	newBranchPath := ".gitre/refs/heads/" + name
	if _, err := os.Stat(newBranchPath); err == nil {
		return fmt.Errorf("branch '%s' already exists", name)
	}
	branches, err := os.ReadDir(".gitre/refs/heads/")
	if err != nil {
		return fmt.Errorf("error finding branches: %w", err)
	}
	head, err := os.ReadFile(".gitre/HEAD")
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	head = bytes.TrimSpace(head)
	currentBranch := strings.TrimPrefix(string(head), "ref: refs/heads/")
	var branchList []string
	fmt.Println("Choose which branch to copy (type number):")
	for i, branch := range branches {
		branchList = append(branchList, branch.Name())
		if branch.Name() == currentBranch {
			fmt.Printf("%d: %s (current)\n", i+1, branch.Name())
		} else {
			fmt.Printf("%d: %s\n", i+1, branch.Name())
		}
	}
	var choice int
	_, err = fmt.Scanln(&choice)
	if err != nil || choice < 1 || choice > len(branches) {
		fmt.Printf("provide valid input (1-%d)\n", len(branches))
		return nil
	}

	hash, err := os.ReadFile(".gitre/refs/heads/" + branches[choice-1].Name())
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	os.WriteFile(".gitre/refs/heads/"+name, hash, 0644)
	os.WriteFile(".gitre/HEAD", []byte("ref: refs/heads/"+name), 0644)

	return nil
}

func switchBranch() error {
	return nil
}

func restore() error {
	return nil
}

func status() error {
	indexEntries, err := LoadIndex()
	indexMap := map[string]string{}
	for _, entry := range indexEntries {
		indexMap[entry.Path] = entry.Hash
	}
	head, err := os.ReadFile(".gitre/HEAD")
	head = bytes.TrimSpace(head)
	currentBranch := strings.TrimPrefix(string(head), "ref: refs/heads/")
	fmt.Printf("On branch: %s\n", currentBranch)
	headHash, err := os.ReadFile(".gitre/refs/heads/" + currentBranch)
	if err != nil || headHash == nil {
		return fmt.Errorf("no commit on current branch: %w", err)
	}
	content, err := ExtractObject(bytes.TrimSpace(headHash))
	firstLine := strings.Split(string(content), "\n")[0]
	treeHash := strings.TrimPrefix(firstLine, "tree ")
	treeObj, _ := ExtractObject([]byte(treeHash))
	headMap := map[string]string{}
	var searchTree func(tree []byte, prefix string)
	searchTree = func(tree []byte, prefix string) {
		lines := strings.SplitSeq(string(tree), "\n")
		for line := range lines {
			if line == "" {
				continue
			}
			splits := strings.Split(line, " ")
			if splits[1] == "tree" {
				treeObj, err := ExtractObject([]byte(splits[2]))
				if err != nil {
					fmt.Printf("%v\n", err)
					return
				}
				searchTree(treeObj, prefix+splits[3]+"/")
			} else {
				headMap[prefix+splits[3]] = splits[2]
			}
		}
	}
	searchTree(treeObj, "")
	fmt.Println("\nSTAGING: (index <-> commit)")
	var mod, new []string
	for k, v := range indexMap {
		value, ok := headMap[k]
		if ok && v != value {
			mod = append(mod, k)
		}
		if !ok {
			new = append(new, k)
		}
		delete(headMap, k)
	}
	fmt.Println("\nModified files:")
	for _, k := range mod {
		fmt.Printf("%s, ", k)
	}
	fmt.Println("\nNew files:")
	for _, k := range new {
		fmt.Printf("%s, ", k)
	}
	fmt.Println("\nDeleted files: ")
	for k := range headMap {
		fmt.Printf("%s, ", k)
	}

	fmt.Println("\nUNSTAGED (disk <-> index): ")
	ignores := accumIgnores()
	diskFiles, err := traverseDir("./", ignores)
	var modified, untracked []string
	for _, file := range diskFiles {
		value, ok := indexMap[file]
		if ok {
			data, _ := os.ReadFile(file)
			fileHash, _ := HashObject(data, "blob")
			if fileHash != value {
				modified = append(modified, file)
			}
		} else {
			untracked = append(untracked, file)
		}
		delete(indexMap, file)
	}
	fmt.Println("\nModified files:")
	for _, k := range modified {
		fmt.Printf("%s, ", k)
	}
	fmt.Println("\nUntracked:")
	for _, k := range untracked {
		fmt.Printf("%s, ", k)
	}
	fmt.Println("\nDeleted files: ")
	for k := range indexMap {
		fmt.Printf("%s, ", k)
	}

	if err != nil {
		return fmt.Errorf("error: %w", err)
	}
	return nil
}
