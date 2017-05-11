package main

import (
	"context"
	"io/ioutil"
	"path/filepath"
)

type SlideOnBase struct {
	Filename string
	Content  []byte
}

func (r *GHRepository) NewPRBaseCommit(ctx context.Context, number int, filename string) (*SlideOnBase, error) {
	sha, err := r.GetPRBaseSHA(ctx, number)
	if err != nil {
		return nil, err
	}
	b, err := r.GetCommitFiles(ctx, r.owner, r.repo, filename, sha)
	if err != nil {
		return nil, err
	}
	return &SlideOnBase{
		Filename: filename,
		Content:  b,
	}, nil
}

func (s *SlideOnBase) ConvertToImage(dest string) error {
	filename := filepath.Join(dest, "base", "slide.pptx")
	err := ioutil.WriteFile(filename, s.Content, 0644)
	if err != nil {
		return err
	}
	return nil
}
