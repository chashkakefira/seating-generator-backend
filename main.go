package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"seating-generator/ga"
	"time"

	"github.com/joho/godotenv"
)

type Response struct {
	Seating []ga.Response
	Fitness int
	Ignored []int
}

func main() {
	log.SetFlags(log.LstdFlags)
	_ = godotenv.Load(".env")
	port := os.Getenv("PORT")
	if port == "" {
		log.Println("No PORT env found, defaulting to 5000")
		port = "5000"
	} else {
		log.Printf("Using PORT from environment: %s", port)
	}
	http.HandleFunc("/generate-seating", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		log.Printf("IN: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		generateSeatingHandler(w, r)

		log.Printf("OUT: Completed in %v", time.Since(start))
	})

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
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
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("JSON Decode Error: %v", err)
		if typeErr, ok := err.(*json.UnmarshalTypeError); ok {
			msg := fmt.Sprintf("Type error: expected %v in field %v, got %v", typeErr.Type, typeErr.Field, typeErr.Value)
			http.Error(w, msg, http.StatusBadRequest)
		} else {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		}
		return
	}
	log.Printf("Task: %d students, %d generations, config %dx%d",
		len(req.Students), req.Generations, req.ClassConfig.Rows, req.ClassConfig.Columns)
	seating, fitness, ignored := ga.RunGA(req)
	if seating == nil {
		log.Println("GA returned nil seating")
		http.Error(w, "Invalid input or no solution found", http.StatusBadRequest)
		return
	}
	log.Printf("Solved: Fitness=%d, Conflicts(Ignored)=%d", fitness, len(ignored))
	response := Response{
		Seating: seating,
		Fitness: fitness,
		Ignored: ignored,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Response Encode Error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
