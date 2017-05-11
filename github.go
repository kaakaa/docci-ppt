package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
)

var client *github.Client

type GHRepository struct {
	owner  string
	repo   string
	client *github.Client
}

func NewRepository(r Repository, token string) *GHRepository {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GHRepository{
		owner:  r.Owner,
		repo:   r.Repo,
		client: client,
	}
}

func (r *GHRepository) ListPullRequestFiles(ctx context.Context, owner string, repo string, number int, opt *github.ListOptions) ([]*github.CommitFile, error) {
	files, resp, err := r.client.PullRequests.ListFiles(ctx, owner, repo, number, opt)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Requesting PR ListFiles error: %v", err)
	}
	return files, nil
}

func (r *GHRepository) GetCommitFiles(ctx context.Context, owner, repo, filename, sha string) ([]byte, error) {
	/** 403 This API returns blobs up to 1 MB in size. The requested blob is too large to fetch via the API,
	    but you can use the Git Data API to request blobs up to 100 MB in size.
		[{Resource:Blob Field:data Code:too_large Message:}]
	**/
	/*
		opt := &github.RepositoryContentGetOptions{
			Ref: sha,
		}
		contents, _, resp, err := r.client.Repositories.GetContents(ctx, owner, repo, filename, opt)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK || contents == nil {
			return nil, fmt.Errorf("Requesting commit files error: %v", err)
		}
		return contents, nil
	*/
	tree, resp, err := r.client.Git.GetTree(ctx, owner, repo, sha, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Requesting commit files error: %v", err)
	}
	for _, e := range tree.Entries {
		if e.GetPath() == filename {
			url := *e.URL
			log.Printf("Requesting blog: %s¥n", url)
			resp, err := http.Get(*e.URL)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("Error occurs when requesting blog for base commit")
			}
			defer resp.Body.Close()

			var blob github.Blob
			decoder := json.NewDecoder(resp.Body)
			err = decoder.Decode(&blob)
			if err != nil {
				return nil, err
			}

			// blob.Content includes Newline string("\n") and cannot be decoded because NewLine string is invalid character for base64.
			// but base64 Decoder of golang base64 pakcage ignore NewLine string, so it's OK.
			c, err := base64.StdEncoding.DecodeString(*blob.Content)
			if err != nil {
				return nil, err
			}
			return c, nil
		}
	}
	return nil, fmt.Errorf("Cannot find target entry")
}

func (r *GHRepository) GetPRBaseSHA(ctx context.Context, number int) (string, error) {
	pr, resp, err := r.client.PullRequests.Get(ctx, r.owner, r.repo, number)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Requesting PullRequest base sha error: %v", err)
	}
	return pr.Base.GetSHA(), nil
}

func (r *GHRepository) MakePullRequest(ctx context.Context, headDir, baseDir, origin string, pr int) error {
	baseBranch := "refs/heads/gh1-test"
	headBranch := "refs/heads/gh1-test-pr"

	base_commit, err := r.makeBranch(ctx, baseBranch, origin, baseDir)
	if err != nil {
		return err
	}
	_, err = r.makeBranch(ctx, headBranch, base_commit.GetSHA(), headDir)
	if err != nil {
		return err
	}

	// make pull request
	npr := &github.NewPullRequest{
		Title: github.String("[docci] Diff PowerPoint"),
		Head:  github.String(fmt.Sprintf("%s:%s", r.owner, headBranch)),
		Base:  github.String(baseBranch),
		Body:  github.String("Diff"),
	}
	prresp, resp, err := r.client.PullRequests.Create(ctx, r.owner, r.repo, npr)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Branch Creating Error")
	}
	log.Printf("%v", prresp)

	return nil
}

func (r *GHRepository) makeBranch(ctx context.Context, name, origin, dirpath string) (*github.Commit, error) {
	// make branch1
	refs, err := r.makeReference(ctx, name, origin)
	if err != nil {
		return nil, err
	}
	// create tree, treeentry, blob
	tree, err := r.makeTree(ctx, dirpath, refs.Object.GetSHA())
	if err != nil {
		return nil, err
	}
	// create commit
	c, err := r.makeCommit(ctx, tree, refs.Object.GetSHA())
	if err != nil {
		return nil, err
	}
	// push
	refs, err = r.pushCommit(ctx, refs, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *GHRepository) makeReference(ctx context.Context, name, parent string) (*github.Reference, error) {
	ref := &github.Reference{
		Ref: github.String(name),
		Object: &github.GitObject{
			SHA: github.String(parent),
		},
	}
	refs, resp, err := r.client.Git.CreateRef(ctx, r.owner, r.repo, ref)
	if err != nil {
		return nil, err
	}
	// TODO: processing when returning 422 Reference already exists
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("Branch Creating Error")
	}
	return refs, nil
}

func (r *GHRepository) makeTree(ctx context.Context, dir, sha string) (*github.Tree, error) {
	var entries []github.TreeEntry
	m, err := filepath.Glob(dir + "/*")
	if err != nil {
		return nil, err
	}
	var entry *github.TreeEntry
	for _, path := range m {
		fInfo, _ := os.Stat(path)
		if fInfo.IsDir() {
			continue
		}
		entry, err = r.createTreeEntry(ctx, dir, path)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	// create tree
	tree, resp, err := r.client.Git.CreateTree(ctx, r.owner, r.repo, sha, entries)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Branch Creating Error")
	}
	return tree, nil

}

func (r *GHRepository) makeCommit(ctx context.Context, tree *github.Tree, sha string) (*github.Commit, error) {
	parent, resp, err := r.client.Git.GetCommit(ctx, r.owner, r.repo, sha)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Branch Creating Error")
	}
	commit := &github.Commit{
		Message: github.String("testhead"),
		Tree:    tree,
		Parents: []github.Commit{*parent},
	}

	c, resp, err := r.client.Git.CreateCommit(ctx, r.owner, r.repo, commit)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Branch Creating Error")
	}
	return c, nil

}

func (r *GHRepository) createTreeEntry(ctx context.Context, basepath, targetpath string) (*github.TreeEntry, error) {
	b, err := ioutil.ReadFile(targetpath)
	if err != nil {
		return nil, err
	}
	content := base64.StdEncoding.EncodeToString(b)
	blob, resp, err := r.client.Git.CreateBlob(ctx, r.owner, r.repo, &github.Blob{
		Content:  github.String(content),
		Encoding: github.String("base64"),
		Size:     github.Int(len(content)),
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Branch Creating Error")
	}

	rel, _ := filepath.Rel(basepath, targetpath)
	return &github.TreeEntry{
		Path: github.String(rel),
		Mode: github.String("100644"),
		Type: github.String("blob"),
		SHA:  blob.SHA,
	}, nil
}

func (r *GHRepository) pushCommit(ctx context.Context, ref *github.Reference, commit *github.Commit) (*github.Reference, error) {
	obj := &github.GitObject{
		Type: github.String("commit"),
		SHA:  commit.SHA,
	}
	ref.Object = obj
	refs, resp, err := r.client.Git.UpdateRef(ctx, r.owner, r.repo, ref, false)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Branch Creating Error")
	}
	return refs, nil
}
