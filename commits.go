package main

import (
	"io"

	"github.com/go-git/go-git/v5/plumbing/object"
)

func CommitsWalk(commits object.CommitIter, fn func(prev, next *object.Commit, patch *object.Patch, i int) error, max int) (int, error) {
	var commitsProcessed int
	var prevCommit *object.Commit

	for {
		if commitsProcessed == max {
			break
		}

		commit, err := commits.Next()
		if err == io.EOF {
			patch, err := prevCommit.Patch(nil)
			if err != nil {
				return commitsProcessed, err
			}

			err = fn(prevCommit, commit, patch, commitsProcessed)
			if err != nil {
				return commitsProcessed, err
			}
			commitsProcessed = commitsProcessed + 1

			break
		}
		if err != nil {
			return commitsProcessed, err
		}

		if prevCommit == nil {
			prevCommit = commit
			continue
		}

		patch, err := prevCommit.Patch(commit)
		if err != nil {
			return commitsProcessed, err
		}

		err = fn(prevCommit, commit, patch, commitsProcessed)
		if err != nil {
			return commitsProcessed, err
		}
		commitsProcessed = commitsProcessed + 1

		prevCommit = commit
	}

	return commitsProcessed, nil
}
