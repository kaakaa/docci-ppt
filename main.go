package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// (opt)Add "Review Slide" label
// -> (1) check whether updating .pptx file
// -> (2) get .pptx file on HEAD
//   => (a) convert .pptx to png
// -> (3) get .pptx file on BASE
//   => (a) convert .pptx to png
// -> (4) make new PullRequest (Title: #XXXX slide review?)
//   => (a) commit BASE png files (.slide dir?) (make new branch?)
//   => (b) commit HEAD png files (.slide dir?)
//   => (c) push PullRequest

func main() {
	conf, err := ReadConfig("./config.json")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Configuration: %v", conf)
	r := NewRepository(conf.PullRequest.Repository, conf.AccessToken)
	ctx := context.Background()

	head, err := r.NewPRHeadCommit(ctx, conf.PullRequest.Number)
	if err != nil {
		log.Fatal(err)
	}
	base, err := r.NewPRBaseCommit(ctx, conf.PullRequest.Number, head.Filename)
	if err != nil {
		log.Fatal(err)
	}

	dir, err := ioutil.TempDir("", "ppt2img")
	defer os.RemoveAll(dir)
	if err != nil {
		log.Fatal(err)
	}
	headDir := filepath.Join(dir, "head")
	os.Mkdir(headDir, 0755)
	err = head.ConvertToImage(dir)
	if err != nil {
		log.Fatal(err)
	}
	baseDir := filepath.Join(dir, "base")
	os.Mkdir(baseDir, 0755)
	err = base.ConvertToImage(dir)
	if err != nil {
		log.Fatal(err)
	}

	r.owner = conf.DestRepository.Owner
	r.repo = conf.DestRepository.Repo
	err = r.MakePullRequest(ctx, headDir, baseDir, "7818ef32a1fe93c34159cde2bf6fc88c8fa75d25", conf.PullRequest.Number)
	log.Println(err)
}
