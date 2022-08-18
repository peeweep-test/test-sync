package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/github"
)

type Config struct {
	Src      string   `json:"src"`
	Dest     string   `json:"dest"`
	Branches []string `json:"branches"`
}
type Branch struct {
	Owner  string
	Repo   string
	Branch string
	Base   string
}

func main() {
	var files, message string
	var dryRun bool
	flag.StringVar(&message, "message", "chore: Sync by .github", "commit message")
	flag.StringVar(&files, "files", "", "config files, separated by spaces")
	flag.BoolVar(&dryRun, "dryRun", false, "dry run")
	flag.Parse()

	if files == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	client := github.NewClient(&http.Client{})
	ctx := context.Background()

	// Sync all repositories if do not repos changed
	for _, file := range strings.Fields(files) {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		var configs []Config
		err = json.Unmarshal(data, &configs)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Config", file)

		for _, config := range configs {
			owner, repo, path, err := split(config.Dest)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("\tSync %s to %s/%s/%s", config.Src, owner, repo, path)
			if dryRun {
				continue
			}
			var syncBranches []string
			branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, nil)
			if err != nil {
				log.Fatal(err)
			}
			// match branch
			for i := range config.Branches {
				reg := regexp.MustCompile(config.Branches[i])
				match := false
				for j := range branches {
					if reg.Match([]byte(*branches[j].Name)) {
						match = true
						break
					}
				}
				if match {
					syncBranches = append(syncBranches, *branches[i].Name)
				}
			}
			// all branch
			if len(config.Branches) == 0 {
				for i := range branches {
					syncBranches = append(syncBranches, *branches[i].Name)
				}
			}
			tmpDir, err := os.MkdirTemp("./", "tmp_")
			if err != nil {
				log.Fatal(err)
			}
			_, err = execCommand(ctx, tmpDir, "git", "clone", fmt.Sprintf("git@github.com:%s/%s.git", owner, repo))
			if err != nil {
				log.Fatal(err)
			}
			workdir := filepath.Join(tmpDir, repo)
			for _, branch := range syncBranches {
				_, err = execCommand(ctx, workdir, "git", "checkout", branch)
				if err != nil {
					log.Fatal(err)
				}
				_, err = execCommand(ctx, workdir, "mkdir", "-p", filepath.Dir(path))
				_, err = execCommand(ctx, workdir, "cp", filepath.Join("../../", config.Src), path)
				if err != nil {
					log.Fatal(err)
				}
				out, err := execCommand(ctx, workdir, "git", "status", "-s")
				if err != nil {
					log.Fatal(err)
				}
				if len(out) == 0 {
					continue
				}
				_, err = execCommand(ctx, workdir, "git", "add", ":/")
				if err != nil {
					log.Fatal(err)
				}
				_, err = execCommand(ctx, workdir, "git", "commit", "-a", "-m", `"`+message+`"`)
				if err != nil {
					log.Fatal(err)
				}
				_, err = execCommand(ctx, workdir, "git", "push")
				if err != nil {
					log.Fatal(err)
				}
			}
		}

	}
}

func execCommand(ctx context.Context, workdir string, command string, args ...string) ([]byte, error) {
	log.Println("\t\texec", "at", workdir, strings.Join(append([]string{command}, args...), " "))
	cmd := exec.Command(command, args...)
	if len(workdir) > 0 {
		cmd.Dir = workdir
	}
	return cmd.CombinedOutput()
}

func split(dest string) (owner, repo, path string, err error) {
	arr := strings.SplitN(dest, "/", 3)
	if len(arr) != 3 {
		return "", "", "", fmt.Errorf("wrong dist. example: owner/repo/file")
	}
	return arr[0], arr[1], arr[2], nil
}
