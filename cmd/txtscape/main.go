package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "content/index.html")
	})

	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "content/style.css")
	})

	mux.HandleFunc("GET /tutorial", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "content/tutorial/index.html")
	})

	mux.HandleFunc("GET /tutorial/{step}", func(w http.ResponseWriter, r *http.Request) {
		step := r.PathValue("step")
		valid := map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true, "6": true}
		if !valid[step] {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "content/tutorial/"+step+".html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("txtscape listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
