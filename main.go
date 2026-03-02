package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Supercloud node is running")
    })
    http.ListenAndServe(":8080", nil)
}
