package main

import (
	//"net/http"
	"encoding/json"
	"log"
	"net/http"
	"seating-generator/ga"
)

type Response struct {
	Seating []ga.Response
	Fitness int64
}

func main() {
	http.HandleFunc("/generate-seating", generateSeatingHandler)
	log.Println("Starting server on localhost:5000")
	if err := http.ListenAndServe(":5000", nil); err != nil {
		log.Fatal(err)
	}
}

func generateSeatingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ga.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	seating, fitness := ga.RunGA(req)
	if seating == nil {
		http.Error(w, "Invalid input or no solution found", http.StatusBadRequest)
		return
	}
	response := Response{
		Seating: seating,
		Fitness: fitness,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
