package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"tagnote/internal/apiclient"
)

func main() {
	if len(os.Args) < 2 {
		listTags()
		return
	}

	switch os.Args[1] {
	case "list":
		listTags()
	case "approve":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tagnote-tags approve <name>")
			os.Exit(1)
		}
		if err := apiclient.ApproveTag(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tag %q approved.\n", os.Args[2])
	case "rename":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: tagnote-tags rename <old-name> <new-name>")
			os.Exit(1)
		}
		if err := apiclient.RenameTag(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tag %q renamed to %q.\n", os.Args[2], os.Args[3])
	case "delete":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tagnote-tags delete <name>")
			os.Exit(1)
		}
		if err := apiclient.DeleteTag(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tag %q deleted.\n", os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: tagnote-tags [list|approve|rename|delete]")
		os.Exit(1)
	}
}

func listTags() {
	tags, err := apiclient.ListTagsDetailed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(tags) == 0 {
		fmt.Println("No tags found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tNOTES")
	fmt.Fprintln(w, "----\t------\t-----")
	for _, t := range tags {
		fmt.Fprintf(w, "%s\t%s\t%d\n", t.Name, t.Status, t.NoteCount)
	}
	w.Flush()
}
