package main

import (
	"fmt"
	"os"
	"sgound/internal/db"
	"sgound/internal/tags"
)

func main() {
	dbPath := "sgound.db"
	masterDB, err := db.ConnectMaster(dbPath)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer masterDB.Conn.Close()

	editor := tags.NewEditor(masterDB)
	trackID := "bdb291226036cf68c9cf3b7b6b2a9cf06814650a"
	fmt.Printf("Testing with track ID: %s\n", trackID)

	// 1. Read current lyrics
	lyrics, err := editor.ReadLyrics(trackID)
	if err != nil {
		fmt.Printf("Error reading lyrics: %v\n", err)
	} else {
		fmt.Printf("Current lyrics: '%s'\n", lyrics)
	}

	// 2. Update lyrics MULTIPLE TIMES
	for i := 1; i <= 3; i++ {
		l := fmt.Sprintf("Lyric version %d", i)
		fmt.Printf("Updating lyrics to: '%s'\n", l)
		err = editor.UpdateLyrics(trackID, l)
		if err != nil {
			fmt.Printf("Error updating lyrics: %v\n", err)
			os.Exit(1)
		}
	}

	// 3. Read again to verify
	lyrics2, err := editor.ReadLyrics(trackID)
	if err != nil {
		fmt.Printf("Error reading lyrics back: %v\n", err)
	} else {
		fmt.Printf("Read back lyrics: '%s'\n", lyrics2)
		if lyrics2 == "Lyric version 3" {
			fmt.Println("✅ SUCCESS: Lyrics match latest version!")
		} else {
			fmt.Printf("❌ FAILURE: Lyrics do not match! Found: '%s', expected 'Lyric version 3'\n", lyrics2)
		}
	}
}
