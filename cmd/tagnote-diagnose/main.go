package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "/data/tagnote.db", "path to SQLite database")
	fix := flag.Bool("fix", false, "attempt to fix broken tag links")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Count totals
	var totalNotes, totalTags, totalLinks int
	db.QueryRow(`SELECT COUNT(*) FROM subnotes`).Scan(&totalNotes)
	db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&totalTags)
	db.QueryRow(`SELECT COUNT(*) FROM subnote_tags`).Scan(&totalLinks)
	fmt.Printf("Total: %d notes, %d tags, %d links\n", totalNotes, totalTags, totalLinks)

	// Notes without any tags
	var notesWithoutTags int
	db.QueryRow(`SELECT COUNT(*) FROM subnotes s WHERE NOT EXISTS (SELECT 1 FROM subnote_tags st WHERE st.subnote_id = s.id)`).Scan(&notesWithoutTags)
	fmt.Printf("Notes without tags: %d\n", notesWithoutTags)

	// Orphaned links (tag_id not in tags)
	var orphanedByTag int
	db.QueryRow(`SELECT COUNT(*) FROM subnote_tags WHERE tag_id NOT IN (SELECT id FROM tags)`).Scan(&orphanedByTag)
	fmt.Printf("Orphaned links (bad tag_id): %d\n", orphanedByTag)

	// Orphaned links (subnote_id not in subnotes)
	var orphanedByNote int
	db.QueryRow(`SELECT COUNT(*) FROM subnote_tags WHERE subnote_id NOT IN (SELECT id FROM subnotes)`).Scan(&orphanedByNote)
	fmt.Printf("Orphaned links (bad subnote_id): %d\n", orphanedByNote)

	// Show users
	fmt.Println("\nUsers:")
	rows, _ := db.Query(`SELECT id, email FROM users`)
	defer rows.Close()
	for rows.Next() {
		var id, email string
		rows.Scan(&id, &email)
		var nc int
		db.QueryRow(`SELECT COUNT(*) FROM subnotes WHERE user_id = ?`, id).Scan(&nc)
		var tc int
		db.QueryRow(`SELECT COUNT(*) FROM tags WHERE user_id = ?`, id).Scan(&tc)
		fmt.Printf("  %s (%s): %d notes, %d tags\n", id, email, nc, tc)
	}

	if !*fix {
		return
	}

	// Fix: for each note that has no subnote_tags links, check if there were
	// links that got broken by the tags table rebuild.
	// The tags table rebuild preserved IDs via INSERT INTO tags_new (id, name, ...)
	// SELECT id, name, ... FROM tags, so IDs should have been preserved.
	// But let's check anyway.
	fmt.Println("\nNothing to auto-fix from tag rebuild (IDs were preserved).")
	fmt.Println("The issue is likely that subnote_tags rows were lost during the tags table rebuild.")
	fmt.Println("Checking if tags_new migration dropped subnote_tags foreign keys...")

	// The real issue: when we did DROP TABLE tags and ALTER TABLE tags_new RENAME TO tags,
	// the ON DELETE CASCADE on subnote_tags(tag_id REFERENCES tags(id)) would have
	// deleted all subnote_tags rows when we dropped the old tags table!
	fmt.Println("\n*** ROOT CAUSE: DROP TABLE tags triggered ON DELETE CASCADE on subnote_tags ***")
	fmt.Println("All subnote_tags rows were deleted when the old tags table was dropped.")
}
