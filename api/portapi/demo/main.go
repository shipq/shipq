package main

import (
	"fmt"
	"net/http"

	"github.com/shipq/shipq/api/portapi/demo/api"
)

func main() {
	mux := api.NewMux()

	addr := ":8080"
	fmt.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
