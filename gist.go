// Copyright (c) 2015 Peter Sanford

package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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
	case "cat":
		if len(os.Args) != 3 {
			usage()
		}
		cat(os.Args[2])
	case "dump-files", "dump":
		dumpToFiles()
	case "create-private":
		if len(os.Args) != 3 {
			usage()
		}
		create(os.Args[2], false)
	case "create-public":
		if len(os.Args) != 3 {
			usage()
		}
		create(os.Args[2], true)
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
	create-private [<filename>|[-]]
	create-public [<filename>|[-]]
	cat <id>
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

func cat(id string) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token()},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	gist, _, err := client.Gists.Get(id)
	if err != nil {
		panic(err)
	}
	for _, gf := range gist.Files {
		if gf.Content != nil {
			fmt.Println(*gf.Content)
		}
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

func create(filename string, public bool) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token()},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	var r io.Reader
	if filename == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		r = f
	}

	content, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}
	var strBody = string(content)

	gistFilename := filepath.Base(filename)
	if filename == "-" {
		h := sha1.New()
		h.Write(content)
		h.Sum(nil)
		gistFilename = fmt.Sprintf("%x", sha1.Sum(nil))
	}

	g := github.Gist{
		Files: map[github.GistFilename]github.GistFile{
			github.GistFilename(gistFilename): github.GistFile{
				Content: &strBody,
			},
		},
		Public: &public,
	}

	created, _, err := client.Gists.Create(&g)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Gist: ", *created.HTMLURL)
}
