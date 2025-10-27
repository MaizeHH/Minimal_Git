package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		commit()
		return
	default:
		fmt.Printf("unknown command: %s. available commands: init, add, commit\n", os.Args[1])
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
		if _, err = os.Create(filepath.Join(repoDir, "index")); err != nil {
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

func add(args []string) {
	fmt.Println("Adding files...")
	obj, _ := HashFile("example.txt")
	fmt.Println("Object hash:", obj)
	return
}

func commit() {
	fmt.Println("Committing changes...")
	return
}
