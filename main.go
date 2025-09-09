package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"seating-generator/ga"

	"github.com/joho/godotenv"
)

type Response struct {
	Seating []ga.Response
	Fitness int64
	Ignored []int
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Could not load .env file, using defaults")
	}
	http.HandleFunc("/generate-seating", generateSeatingHandler)
	log.Println("Starting server...")
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}

func generateSeatingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", os.Getenv("ALLOWED_ORIGIN"))
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ga.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	seating, fitness, ignored := ga.RunGA(req)
	if seating == nil {
		http.Error(w, "Invalid input or no solution found", http.StatusBadRequest)
		return
	}
	response := Response{
		Seating: seating,
		Fitness: fitness,
		Ignored: ignored,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
