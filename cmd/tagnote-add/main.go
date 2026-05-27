package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/runminglu/tag-note/internal/apiclient"
)

type tagList []string

func (t *tagList) String() string { return strings.Join(*t, ", ") }
func (t *tagList) Set(val string) error {
	*t = append(*t, val)
	return nil
}

func main() {
	var tags tagList
	flag.Var(&tags, "t", "tag (can be repeated)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: tagnote-add -t <tag> [-t <tag>] \"content\"")
		os.Exit(1)
	}
	content := strings.Join(args, " ")

	if len(tags) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one tag (-t) is required")
		os.Exit(1)
	}

	resp, err := apiclient.CreateNote(content, tags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created: %s (at %s)\n", resp.ShortID, resp.CreatedAt.Format("2006-01-02 15:04:05"))
}
