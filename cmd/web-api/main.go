package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"os/signal"
	"sync"
	"syscall"

	"github.com/mohammadne/ice-global/cmd"
	"github.com/mohammadne/ice-global/internal/api/http"
	"github.com/mohammadne/ice-global/internal/db"
)

func main() {
	httpPort := flag.Int("http-port", 8088, "The server port which handles http requests (default: 8088)")
	flag.Parse() // Parse the command-line flags

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelInfo})))
	cmd.BuildInfo()

	// cfg, err := config.LoadDefaults(true, "")
	// if err != nil {
	// 	panic(err)
	// }

	// mysql, err := mysql.Open(cfg.Mysql, "")
	// if err != nil {
	// 	slog.Error(`error connecting to mysql database`, `Err`, err)
	// 	os.Exit(1)
	// }
	// _ = mysql
	db.MigrateDatabase()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var wg sync.WaitGroup

	wg.Add(1)
	go http.New().Serve(ctx, &wg, *httpPort)

	<-ctx.Done()
	wg.Wait()
	slog.Warn("interruption signal recieved, gracefully shutdown the server")
}
