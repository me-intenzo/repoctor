package main

type Check interface {
	Name() string
	Run(repoPath string) ([]Finding, error)
}