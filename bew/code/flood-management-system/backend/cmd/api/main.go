package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"flood-management-system/backend/internal/api"
	"flood-management-system/backend/internal/data"
	"flood-management-system/backend/internal/store"
)

const defaultSource = "https://api-v3.thaiwater.net/api/v1/thaiwater30/public/waterlevel_load"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := store.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer db.DB.Close()
	sourceURL := os.Getenv("THAIWATER_URL")
	if sourceURL == "" {
		sourceURL = defaultSource
	}
	client := &http.Client{Timeout: 20 * time.Second}
	graphURL := os.Getenv("THAIWATER_GRAPH_URL")
	if graphURL == "" {
		graphURL = data.DefaultGraphURL
	}
	go func() {
		syncOnce(ctx, db, client, sourceURL)
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncOnce(ctx, db, client, sourceURL)
			}
		}
	}()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: api.Router(db, graphURL), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("API listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func syncOnce(ctx context.Context, db *store.Store, client *http.Client, url string) {
	attempt := time.Now().UTC()
	readings, err := data.FetchLatest(ctx, client, url)
	if err == nil {
		err = db.SaveReadings(ctx, readings)
	}
	count := 0
	if err == nil {
		count = len(readings)
	}
	if recordErr := db.RecordSync(ctx, attempt, count, err); recordErr != nil {
		log.Printf("record source sync: %v", recordErr)
	}
	if err != nil {
		log.Printf("source sync failed: %v", err)
	} else {
		log.Printf("source sync saved %d readings", count)
	}
}
