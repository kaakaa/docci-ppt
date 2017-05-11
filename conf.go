package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	PullRequest    PullRequest `json:"pull_request"`
	DestRepository Repository  `json:"dest"`
	AccessToken    string      `json:"access_token"`
}

type PullRequest struct {
	Repository Repository
	Number     int
}

type Repository struct {
	Owner string
	Repo  string
}

func ReadConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
