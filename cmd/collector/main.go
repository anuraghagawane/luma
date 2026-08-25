package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Luma started...")
	http.HandleFunc("/v1/log", logHandler)
	http.HandleFunc("/v1/bulklog", bulkLogHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type LogLevel string

const (
	ERROR LogLevel = "ERROR"
	DEBUG LogLevel = "DEBUG"
)

func (loglevel *LogLevel) UnmarshalText(text []byte) error {
	str := LogLevel(text)
	switch str {
	case ERROR, DEBUG:
		*loglevel = str
		return nil
	default:
		return fmt.Errorf("invalid loglevel: %s", string(text))
	}
}

type Log struct {
	EventID  string   `json:"eventid"`
	Tenant   string   `json:"tenant"`
	Host     string   `json:"host"`
	LogLevel LogLevel `json:"loglevel"`
	Message  string   `json:"message"`
}

func logHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var log Log
		data, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(data, &log)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}
		fmt.Printf("%+v\n", log)
		fmt.Fprintf(w, "Yo, reponse form luma")
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}

func bulkLogHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var logs []Log
		data, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(data, &logs)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}
		fmt.Printf("%+v\n", logs)
		fmt.Fprintf(w, "Yo, reponse form luma")
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}
