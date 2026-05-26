package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"tagnote/internal/apiclient"
)

type tagList []string

func (t *tagList) String() string { return strings.Join(*t, ", ") }
func (t *tagList) Set(val string) error {
	*t = append(*t, val)
	return nil
}

func main() {
	var tags tagList
	search := flag.String("s", "", "search query (full-text search)")
	flag.Var(&tags, "t", "tag (can be repeated)")
	flag.Parse()

	if len(tags) == 0 && *search == "" {
		fmt.Fprintln(os.Stderr, "usage: tagnote-read -t <tag> [-t <tag>] [-s <query>]")
		os.Exit(1)
	}

	stream, err := apiclient.ReadStream(tags, *search)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *search != "" {
		re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(*search))
		if err == nil {
			stream = re.ReplaceAllStringFunc(stream, func(m string) string {
				return "\033[1;33m" + m + "\033[0m"
			})
		}
	}

	fmt.Print(stream)
}
