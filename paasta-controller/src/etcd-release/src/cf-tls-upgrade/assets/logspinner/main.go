package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")

		switch string(parts[1]) {
		case "log":
			if len(parts) <= 2 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fmt.Printf("[%s] %s\n", time.Now(), parts[2])
		}
		w.WriteHeader(http.StatusOK)
	})

	fmt.Println("starting logspinner")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}
