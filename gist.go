// Copyright (c) 2015, RetailNext, Inc.
// This material contains trade secrets and confidential information of
// RetailNext, Inc.  Any use, reproduction, disclosure or dissemination
// is strictly prohibited without the explicit written permission
// of RetailNext, Inc.
// All rights reserved.

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	arg := os.Args[1]

	switch arg {
	case "list":
		list()
	case "dump-files", "dump":
		dumpToFiles()
	case "grep":
		if len(os.Args) < 3 {
			usage()
		}
		grep(strings.Join(os.Args[2:], " "))
	default:
		usage()
	}
}

var usageString = `Usage:

	gist command

The commands are:

	list
	dump-files
	grep <pattern>

`

func usage() {
	fmt.Fprintln(os.Stderr, usageString)
	os.Exit(1)
}

func list() {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token()},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	opts := &github.GistListOptions{}
	for {
		// list all repositories for the authenticated user
		gists, resp, err := client.Gists.List("", opts)

		if err != nil {
			panic(err)
		}

		for _, g := range gists {
			var desc string
			if g.Description != nil {
				desc = *g.Description
			}
			fmt.Printf("%s %s %s\n", g.CreatedAt.Format("2006-02-01"), *g.ID, desc)
			var files []string
			for name, _ := range g.Files {
				files = append(files, string(name))
			}
			fmt.Println("  ", strings.Join(files, ","))
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
}

func dumpToFiles() {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token()},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	opts := &github.GistListOptions{}

	for {
		// list all repositories for the authenticated user
		gists, resp, err := client.Gists.List("", opts)

		if err != nil {
			panic(err)
		}

		var wg sync.WaitGroup

		for _, g := range gists {
			wg.Add(1)
			go func(g github.Gist) {
				gist, _, err := client.Gists.Get(*g.ID)
				if err != nil {
					panic(err)
				}
				err = os.MkdirAll(filepath.Join("/tmp/gists", *g.ID), 0700)
				if err != nil {
					panic(err)
				}
				for name, gf := range gist.Files {
					f, err := os.Create(filepath.Join(filepath.Join("/tmp/gists", *g.ID, string(name))))
					if err != nil {
						panic(err)
					}
					if gf.Content != nil {
						io.WriteString(f, *gf.Content)
					}
					f.Close()
				}
				wg.Done()
			}(g)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
}

func grep(match string) {
	upMatch := strings.ToUpper(match)
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token()},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	opts := &github.GistListOptions{}

	results := make(chan *github.Gist)
	var wg sync.WaitGroup

	go func() {
		for g := range results {
			var desc string
			if g.Description != nil {
				desc = *g.Description
			}
			fmt.Printf("%s %s %s\n", g.CreatedAt.Format("2006-02-01"), *g.ID, desc)
			for name, f := range g.Files {
				fmt.Println("  ", name)
				if f.Content != nil {
					fmt.Println(*f.Content)
				}
				fmt.Println("")
			}
		}
	}()

	for {
		// list all repositories for the authenticated user
		gists, resp, err := client.Gists.List("", opts)

		if err != nil {
			panic(err)
		}

		for _, g := range gists {
			wg.Add(1)
			go func(g github.Gist) {
				gist, _, err := client.Gists.Get(*g.ID)
				if err != nil {
					panic(err)
				}
			OUTER:
				for _, gf := range gist.Files {
					if gf.Content != nil {
						content := strings.ToUpper(*gf.Content)
						if strings.Contains(content, upMatch) {
							results <- gist
							break OUTER
						}
					}
				}
				wg.Done()
			}(g)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
}

func token() string {
	out, err := exec.Command("git", "config", "gist.token").Output()
	if err != nil {
		panic(err)
	}
	if out == nil {
		panic("No gist.token configured in gitconfig")
	}
	return string(out)
}
