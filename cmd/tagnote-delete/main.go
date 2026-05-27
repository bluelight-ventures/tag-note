package main

import (
	"fmt"
	"os"

	"github.com/runminglu/tag-note/internal/apiclient"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tagnote-delete <id>")
		os.Exit(1)
	}
	id := os.Args[1]

	if err := apiclient.DeleteNote(id); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted: %s\n", id)
}
