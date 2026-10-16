package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"flood-management-system/backend/internal/data"
	"flood-management-system/backend/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: import-csv path/to/history.csv|https://source/history.csv")
	}
	var input = os.Args[1]
	var reader interface {
		Read([]byte) (int, error)
		Close() error
	}
	if strings.HasPrefix(input, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		response, err := client.Get(input)
		if err != nil {
			log.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			log.Fatalf("CSV URL returned HTTP %d", response.StatusCode)
		}
		reader = response.Body
	} else {
		file, err := os.Open(input)
		if err != nil {
			log.Fatal(err)
		}
		reader = file
	}
	defer reader.Close()
	rows, err := data.DecodeHistoricalCSV(reader)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.DB.Close()
	count, err := db.ImportHistory(ctx, rows, "ThaiWater historical CSV")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("imported %d new observations from %d valid rows", count, len(rows))
}
