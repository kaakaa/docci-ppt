package main

type SlideCommit interface {
	ConvertToImage(dest string) error
}
