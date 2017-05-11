package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/google/go-github/github"
)

type SlideOnHead struct {
	Filename string
	Content  []byte
	Status   string
}

func (r *GHRepository) NewPRHeadCommit(ctx context.Context, number int) (*SlideOnHead, error) {
	files, err := r.ListPullRequestFiles(ctx, r.owner, r.repo, number, nil)
	if err != nil {
		return nil, err
	}
	return getSlide(files)
}

func getSlide(files []*github.CommitFile) (*SlideOnHead, error) {
	// TODO: Need to implement processes when There are multi .pptx files
	for _, f := range files {
		name := f.GetFilename()
		ext := filepath.Ext(name)
		if ext == ".pptx" {
			url := f.GetRawURL()
			resp, err := http.Get(url)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("Error occurs when requesting to %s", url)
			}
			defer resp.Body.Close()
			d, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return &SlideOnHead{
				Filename: name,
				Content:  d,
				Status:   f.GetStatus(), // TODO: This app should start processing if only status == "modified"
			}, nil
		}
	}
	return nil, fmt.Errorf("Cannnot find .pptx file on PR Changes")
}

func (s *SlideOnHead) ConvertToImage(dest string) error {
	filename := filepath.Join(dest, "head", "slide.pptx")
	err := ioutil.WriteFile(filename, s.Content, 0644)
	if err != nil {
		return err
	}
	return nil
}
