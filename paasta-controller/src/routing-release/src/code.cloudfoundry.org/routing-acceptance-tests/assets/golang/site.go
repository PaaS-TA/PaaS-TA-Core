package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/", hello)
	port := os.Getenv("PORT")
	fmt.Printf("Listening on %s...", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Println("Recieved request ", time.Now())
	fmt.Fprintln(res, "go, world")
}
