package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

const legacyUserID = "00000000000000000000000000"

func main() {
	dbPath := flag.String("db", "/data/tagnote.db", "path to SQLite database")
	toEmail := flag.String("to", "", "email of the target user to migrate data to")
	dryRun := flag.Bool("dry-run", false, "show what would be done without making changes")
	flag.Parse()

	if *toEmail == "" {
		fmt.Fprintln(os.Stderr, "usage: tagnote-migrate -to <email>")
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Find target user
	var targetID string
	err = db.QueryRow(`SELECT id FROM users WHERE email = ?`, *toEmail).Scan(&targetID)
	if err == sql.ErrNoRows {
		log.Fatalf("user with email %q not found", *toEmail)
	}
	if err != nil {
		log.Fatalf("find user: %v", err)
	}
	fmt.Printf("Target user: %s (%s)\n", targetID, *toEmail)

	// Count legacy data
	var noteCount, tagCount int
	db.QueryRow(`SELECT COUNT(*) FROM subnotes WHERE user_id = ?`, legacyUserID).Scan(&noteCount)
	db.QueryRow(`SELECT COUNT(*) FROM tags WHERE user_id = ?`, legacyUserID).Scan(&tagCount)
	fmt.Printf("Legacy data: %d notes, %d tags\n", noteCount, tagCount)

	if noteCount == 0 && tagCount == 0 {
		fmt.Println("No legacy data to migrate.")
		return
	}

	if *dryRun {
		fmt.Println("Dry run — no changes made.")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Migrate tags: for each legacy tag, either merge into existing user tag or reassign
	rows, err := tx.Query(`SELECT id, name FROM tags WHERE user_id = ?`, legacyUserID)
	if err != nil {
		log.Fatalf("list legacy tags: %v", err)
	}
	var legacyTags []struct {
		id   int
		name string
	}
	for rows.Next() {
		var t struct {
			id   int
			name string
		}
		rows.Scan(&t.id, &t.name)
		legacyTags = append(legacyTags, t)
	}
	rows.Close()

	for _, lt := range legacyTags {
		// Check if target user already has a tag with this name
		var existingID int
		err := tx.QueryRow(`SELECT id FROM tags WHERE name = ? AND user_id = ?`, lt.name, targetID).Scan(&existingID)
		if err == sql.ErrNoRows {
			// Simple reassign
			tx.Exec(`UPDATE tags SET user_id = ? WHERE id = ?`, targetID, lt.id)
			fmt.Printf("  tag %q: reassigned\n", lt.name)
		} else if err == nil {
			// Merge: point subnote_tags from old tag to existing tag, then delete old tag
			tx.Exec(`UPDATE OR IGNORE subnote_tags SET tag_id = ? WHERE tag_id = ?`, existingID, lt.id)
			tx.Exec(`DELETE FROM subnote_tags WHERE tag_id = ?`, lt.id)
			tx.Exec(`DELETE FROM tags WHERE id = ?`, lt.id)
			fmt.Printf("  tag %q: merged into existing\n", lt.name)
		} else {
			log.Fatalf("check existing tag: %v", err)
		}
	}

	// Migrate subnotes
	res, err := tx.Exec(`UPDATE subnotes SET user_id = ? WHERE user_id = ?`, targetID, legacyUserID)
	if err != nil {
		log.Fatalf("migrate notes: %v", err)
	}
	affected, _ := res.RowsAffected()
	fmt.Printf("  notes: %d reassigned\n", affected)

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Println("Migration complete.")
}
