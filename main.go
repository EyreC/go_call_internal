package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func newHttpClient(timeout int) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          256,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: time.Duration(timeout) * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}
}

type User struct {
	Id   string
	Name string
}

var httpClient *http.Client

func getUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	baseUrl := os.Getenv("INTERNAL_GO_URL")
	internalPortNumber := os.Getenv("INTERNAL_GO_PORT")
	internalPort := ":" + internalPortNumber
	resp, httpErr := httpClient.Get(baseUrl + internalPort + "/users/" + id)
	if httpErr != nil {
		http.Error(w, httpErr.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	bytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		http.Error(w, readErr.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("content-type", "application/json")
	w.Write(bytes)
}
func createServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET users/{id}", getUser)
	portNumber := os.Getenv("PORT")
	port := ":" + portNumber
	server := http.Server{
		Addr:              port,
		Handler:           mux,
		ReadTimeout:       time.Duration(5) * time.Second,
		ReadHeaderTimeout: time.Duration(2) * time.Second,
		WriteTimeout:      time.Duration(15) * time.Second,
		IdleTimeout:       time.Duration(120) * time.Second,
	}
	return &server
}
func main() {
	// make a http request to the internal private network service
	httpClient = newHttpClient(90)
	server := createServer()
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
