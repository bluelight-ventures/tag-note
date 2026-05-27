package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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
	search := flag.String("s", "", "search query (full-text search)")
	flag.Var(&tags, "t", "tag (can be repeated)")
	flag.Parse()

	notes, err := apiclient.ListNotes(tags, *search)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(notes) == 0 {
		fmt.Println("No notes found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTIMESTAMP\tTAGS\tSNIPPET")
	fmt.Fprintln(w, "--\t---------\t----\t-------")
	for _, n := range notes {
		snippet := n.Content
		if n.Snippet != "" {
			snippet = n.Snippet
		} else if len(snippet) > 60 {
			snippet = snippet[:57] + "..."
		}
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		// Highlight FTS matches with bold yellow ANSI codes
		snippet = strings.ReplaceAll(snippet, "[[", "\033[1;33m")
		snippet = strings.ReplaceAll(snippet, "]]", "\033[0m")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			n.ShortID,
			n.CreatedAt.Format("2006-01-02 15:04:05"),
			strings.Join(n.Tags, ","),
			snippet,
		)
	}
	w.Flush()
}
