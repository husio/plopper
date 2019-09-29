package main

import (
	"log"
	"net/http"
	"os"

	"github.com/husio/plopper/lith"
	"github.com/husio/plopper/plopper"
)

func main() {
	conf := struct {
		Port     string
		AuthAPI  string
		AuthUI   string
		Database string
	}{
		Port:     env("PORT", "8000"),
		AuthAPI:  env("LITH_API", "https://lith-demo.herokuapp.com/api"),
		AuthUI:   env("LOGIN_URL", "https://lith-demo.herokuapp.com/pub/"),
		Database: env("DATABASE", "/tmp/plopper.sqlite3"),
	}

	log.SetOutput(os.Stderr)
	log.Printf("Running HTTP server on port %s", conf.Port)

	auth := lith.NewClient(conf.AuthAPI, &http.Client{Transport: requestLogger{}})

	plopStore, err := plopper.OpenSQLitePlopStore(conf.Database)
	if err != nil {
		log.Fatalf("cannot open plops store: %s", err)
	}
	defer plopStore.Close()

	http.Handle("/", plopper.NewHTTPApplication(plopStore, auth, conf.AuthUI))

	if err := http.ListenAndServe(":"+conf.Port, nil); err != nil {
		log.Fatalf("http server: %s", err)
	}
}

func env(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}

type requestLogger struct{}

func (requestLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		log.Printf("request to %q results in error %s", req.URL, err)
	} else {
		log.Printf("request to %q results in status code %d", req.URL, resp.StatusCode)
	}
	return resp, err
}
