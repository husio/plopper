package main

import (
	"log"
	"net/http"
	"os"

	"github.com/husio/shitter/sht"
)

func main() {
	log.SetOutput(os.Stderr)
	port := env("PORT", "8000")
	log.Printf("Running HTTP server on port %s", port)

	plopStore, err := sht.OpenSQLitePlopStore("plops.sqlite3.db")
	if err != nil {
		log.Fatalf("cannot open plops store: %s", err)
	}
	defer plopStore.Close()

	http.Handle("/", sht.NewHTTPApplication(plopStore))

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("http server: %s", err)
	}
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}
