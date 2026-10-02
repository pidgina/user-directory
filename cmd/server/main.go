package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"proj/server"
	"proj/storage"
)

func main() {
	addr := flag.String("addr", ":8080", "адрес сервера")
	dbPath := flag.String("db", "users.db", "файл базы данных")
	flag.Parse()

	openCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	st, err := storage.Open(openCtx, *dbPath)
	cancel()
	if err != nil {
		log.Fatalf("БД: %v", err)
	}
	defer st.Close()

	srv, err := server.New(st)
	if err != nil {
		log.Fatalf("сервер: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("остановка: %v", err)
		}
	}()

	log.Printf("слушаю http://localhost%s", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("сервер: %v", err)
	}
}
