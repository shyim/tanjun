package main

import "net/http"

func main() {
	// create a basic http server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello Deployment"))
	})

	http.ListenAndServe(":3000", nil)
}
