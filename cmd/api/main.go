package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"school-management/internal/database"
	"school-management/internal/handlers"
	"school-management/web"
)

// version is set at build time: -ldflags "-X main.version=v1.0.0".
var version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /readyz and exit 0 if ready (used by the container health check)")
	backup := flag.String("backup", "", "write a verified snapshot of the database to this path and exit")
	flag.Parse()

	level := slog.LevelInfo
	if env("LOG_LEVEL", "info") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	handlers.Version = version

	addr := env("ADDR", ":8080")

	if *healthcheck {
		c := http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1" + addr + "/readyz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}

	dbPath := env("DB_PATH", "school.db")
	db, err := database.Open(dbPath)
	if err != nil {
		fatal("open database", err)
	}
	defer db.Close()

	if *backup != "" {
		if err := database.Backup(db, *backup); err != nil {
			fatal("backup", err)
		}
		slog.Info("backup written", "path", *backup)
		return
	}

	if env("SEED_DATA", "true") == "true" {
		if err := database.Seed(db, time.Now()); err != nil {
			fatal("seed", err)
		}
	}

	mux := http.NewServeMux()
	(&handlers.Handler{DB: db}).Routes(mux)
	mux.Handle("/", http.FileServerFS(web.Files))

	srv := &http.Server{Addr: addr, Handler: handlers.RequestLog(mux), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()
	slog.Info("starting", "addr", addr, "version", version, "db", dbPath)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fatal("server", err)
	}
}
