package main

import (
	"fmt"
	"log"
	"net/http"
)

// params containing reuqest information and response writer to write the response back to the client
func healthHandler(w http.ResponseWriter, r *http.Request) { 
	fmt.Fprintf(w, "ok!")
}

func main(){
	http.HandleFunc("/health", healthHandler) // route for health check
	// start the server on port 8080 & used log.Fatal to log any errors that occur while starting the server
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil)) 
}