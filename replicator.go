package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	defaultEditor          = "vi"
	defaultNumberOfCommits = 5
)

type (
	patchToApply struct {
		Name  string
		Files []string
	}
)

func main() {
	rootFlagSet := flag.NewFlagSet("git-replicator", flag.ExitOnError)

	var (
		commitsNumber                 int
		localRepo, branchName, remote string
	)
	rootFlagSet.StringVar(&localRepo, "l", "", "Path to local repository to replicate from")
	rootFlagSet.StringVar(&branchName, "b", "main", "Repository branch name to replicate from")
	rootFlagSet.StringVar(&remote, "r", "origin", "Repository remote")
	rootFlagSet.IntVar(&commitsNumber, "n", 0, "Number of commits to replicate")

	err := rootFlagSet.Parse(os.Args[1:])
	assertFatalError(err)

	args := rootFlagSet.Args()
	argsLen := len(args)

	if argsLen == 0 && localRepo == "" {
		log.Fatal("no repository specified")
	}

	var stringURL string
	if len(args) > 0 {
		stringURL = args[0]
	}

	urlParts, err := url.Parse(stringURL)
	assertFatalError(err)

	repo := localRepo
	repoURL := stringURL

	token := os.Getenv("GIT_AUTH_TOKEN")

	if urlParts.Host == githubHost && strings.Contains(urlParts.Path, githubPullsPath) {
		if token == "" {
			log.Fatal("Github access token is empty")
		}

		gh := NewGithub(token)
		prData, err := gh.GetPullRequestData(stringURL)
		assertFatalError(err)

		branchName = prData.BranchName
		repo = prData.RepoName
		repoURL = prData.RepoURL
		commitsNumber = prData.Commits
	}

	if commitsNumber == 0 {
		commitsNumber = defaultNumberOfCommits
	}

	home, err := homeDir()
	assertFatalError(err)

	repoStore := filepath.Join(home, "/repositories")
	err = os.MkdirAll(repoStore, 0o755)
	assertFatalError(err)

	wd, err := os.Getwd()
	assertFatalError(err)

	currentRepo, err := git.PlainOpen(wd)
	assertFatalError(err)

	workTree, err := currentRepo.Worktree()
	assertFatalError(err)

	var r *git.Repository

	if repo == "" {
		pathParts := strings.Split(urlParts.Path, "/")
		repoName := pathParts[len(pathParts)-1]

		repo = repoName
	}

	if repoURL != "" {
		repoPath := filepath.Join(repoStore, repo)

		if _, err := os.Stat(repoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
		}
		if err == nil {
			err := os.RemoveAll(repoPath)
			assertFatalError(err)
		}

		cloneCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		if token == "" {
			log.Fatal("Github access token is empty")
		}

		r, err = git.PlainCloneContext(cloneCtx, repoPath, false, &git.CloneOptions{
			URL: repoURL,
			Auth: &http.BasicAuth{
				Username: "test", // yes, it can be anything :)
				Password: token,
			},
		})
		assertFatalError(err)

		fetchContext, fetchCancel := context.WithTimeout(context.Background(), time.Second*10)
		defer fetchCancel()

		err = r.FetchContext(fetchContext, &git.FetchOptions{
			RemoteName: remote,
			RefSpecs:   []config.RefSpec{"refs/*:refs/*"},
		})
		assertFatalError(err)
	} else {
		r, err = git.PlainOpen(repo)
		assertFatalError(err)
	}

	wt, err := r.Worktree()
	assertFatalError(err)

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Force:  true,
	})
	assertFatalError(err)

	localCommits, err := r.Log(&git.LogOptions{})
	assertFatalError(err)
	defer localCommits.Close()

	filePatches := make([]patchToApply, commitsNumber)
	commits := make([]*object.Commit, commitsNumber)

	commitsToPickTmp, err := os.CreateTemp(home, "commits")
	assertFatalError(err)
	defer os.Remove(commitsToPickTmp.Name())

	var commitsAdded int

	var prevCommit *object.Commit
	var prevTree *object.Tree
	for {
		if commitsAdded == commitsNumber {
			break
		}

		commit, err := localCommits.Next()
		if commit == nil {
			commitMessageCut := strings.Split(strings.ReplaceAll(prevCommit.Message, "\r\n", "\n"), "\n")[0]
			_, err = commitsToPickTmp.WriteString(fmt.Sprintf("pick %s %s\n", prevCommit.Hash, commitMessageCut))
			assertFatalError(err)

			patchName := filepath.Join(wd, fmt.Sprintf("%s.patch", prevCommit.Hash))
			p := patchToApply{
				Name: patchName,
			}
			file, err := os.Create(patchName)
			assertFatalError(err)

			patch, err := prevCommit.Patch(nil)
			assertFatalError(err)

			err = patch.Encode(file)
			assertFatalError(err)

			for _, fp := range patch.FilePatches() {
				from, to := fp.Files()

				if from != nil {
					p.Files = append(p.Files, from.Path())
				}
				if to != nil {
					p.Files = append(p.Files, to.Path())
				}
			}
			filePatches[commitsNumber-commitsAdded-1] = p
			commits[commitsNumber-commitsAdded-1] = prevCommit
			commitsAdded = commitsAdded + 1

			err = file.Close()
			assertFatalError(err)

			break
		}
		assertFatalError(err)

		tree, err := commit.Tree()
		assertFatalError(err)

		if prevCommit == nil {
			prevCommit = commit
			prevTree = tree
			continue
		}

		commitMessageCut := strings.Split(strings.ReplaceAll(prevCommit.Message, "\r\n", "\n"), "\n")[0]
		_, err = commitsToPickTmp.WriteString(fmt.Sprintf("pick %s %s\n", prevCommit.Hash, commitMessageCut))
		assertFatalError(err)

		changes, err := tree.Diff(prevTree)
		assertFatalError(err)

		patchName := filepath.Join(wd, fmt.Sprintf("%s.patch", prevCommit.Hash))
		p := patchToApply{
			Name: patchName,
		}
		file, err := os.Create(patchName)
		assertFatalError(err)

		for _, c := range changes {
			patch, err := c.Patch()
			assertFatalError(err)

			err = patch.Encode(file)
			assertFatalError(err)

			for _, fp := range patch.FilePatches() {
				_, f := fp.Files()
				p.Files = append(p.Files, f.Path())
			}
		}
		filePatches[commitsNumber-commitsAdded-1] = p
		commits[commitsNumber-commitsAdded-1] = prevCommit
		commitsAdded = commitsAdded + 1

		err = file.Close()
		assertFatalError(err)

		prevCommit = commit
		prevTree = tree
	}

	_, err = commitsToPickTmp.WriteString(_message)
	assertFatalError(err)

	err = commitsToPickTmp.Sync()
	assertFatalError(err)

	err = commitsToPickTmp.Close()
	assertFatalError(err)

	fmt.Printf("We are about to replicate %d commits, proceed? yes / no? ", commitsAdded)

	s := readUserInput()
	if s == "no" {
		err := editFile(commitsToPickTmp.Name())
		assertFatalError(err)
	}

	file, err := os.Open(commitsToPickTmp.Name())
	assertFatalError(err)

	scanner := bufio.NewScanner(file)
	commitsMap := make(map[string]bool)

	for scanner.Scan() {
		text := scanner.Text()
		textSlice := strings.Split(text, " ")

		fmt.Println(text)

		action := textSlice[0]
		if action != "pick" && action != "drop" {
			break
		}

		var toPick bool
		if action == "pick" {
			toPick = true
		}

		commitHash := textSlice[1]
		commitsMap[commitHash] = toPick
	}
	err = file.Close()
	assertFatalError(err)

	filePatches = filePatches[(commitsNumber - commitsAdded):commitsNumber]
	for i, commit := range commits[(commitsNumber - commitsAdded):commitsNumber] {
		teardown := func() {
			err := os.Remove(filePatches[i].Name)
			assertFatalError(err)
		}

		if !commitsMap[commit.Hash.String()] {
			teardown()
			continue
		}

		if err := applyPatch(filePatches[i].Name); err != nil {
			fmt.Printf("commit %s failed to apply, edit patch file? yes / no ? ", commit.Hash.String())
			s := readUserInput()

			if s == "yes" {
				err := editFile(filePatches[i].Name)
				assertFatalError(err)

				if err := applyPatch(filePatches[i].Name); err != nil {
					fmt.Printf("error occured during git apply %s\n", err)
					teardown()
					continue
				}
			}
			teardown()
			continue
		}

		for _, f := range filePatches[i].Files {
			_, err = workTree.Add(f)
			if err != nil {
				fmt.Printf("failed to add file %s: %s\n", f, err)
				continue
			}
		}

		_, err = workTree.Commit(commit.Message, &git.CommitOptions{})
		assertFatalError(err)

		teardown()
	}
}

func homeDir() (home string, err error) {
	home = os.Getenv("GITREPLICATOR_HOME")
	if home != "" {
		return
	}

	home, err = os.UserHomeDir()
	if err != nil {
		return
	}

	home = filepath.Join(home, ".gitreplicator")
	return
}

func readUserInput() string {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		return scanner.Text()
	}

	return ""
}

func assertFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getEditor() string {
	editor := os.Getenv("GIT_EDITOR")

	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = defaultEditor
	}

	return editor
}

func applyPatch(patch string) error {
	cmd := exec.Command("git", "apply", patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func editFile(path string) error {
	cmd := exec.Command(getEditor(), path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
