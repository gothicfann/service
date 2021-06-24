package main

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/pkg/errors"
)

var build = "develop"

func main() {
	log := log.New(os.Stdout, "SALES: ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
	if err := run(log); err != nil {
		log.Println("main: error:", err)
		os.Exit(1)
	}
}

func run(log *log.Logger) error {

	//===================================================================
	// Configuration

	var cfg struct {
		conf.Version
		Web struct {
			APIHost         string        `conf:"default:0.0.0.0:3000"`
			DebugHost       string        `conf:"default:0.0.0.0:4000"`
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:5s"`
			ShutdownTimeout time.Duration `conf:"default:5s"`
		}
	}

	cfg.Version.SVN = build
	cfg.Version.Desc = "Copyright information here"

	if err := conf.Parse(os.Args[1:], "SALES", &cfg); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			usage, err := conf.Usage("SALES", &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config usage")
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString("SALES", &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}
			fmt.Println(version)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}

	//===================================================================
	// App Starting
	expvar.NewString("build").Set(build)
	log.Printf("main: Started : Application initializing : version %q", build)
	defer log.Println("main: Completed")

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config output")
	}
	log.Printf("main: Config :\n%v\n", out)

	//===================================================================
	// Start Debug Service
	log.Println("main: Initializing debugging support")

	go func() {
		log.Printf("main: Debug Listening %s", cfg.Web.DebugHost)
		if err := http.ListenAndServe(cfg.Web.DebugHost, http.DefaultServeMux); err != nil {
			log.Printf("main: Debug Listener closed : %v", err)
		}
	}()

	//===================================================================
	// Start API Service

	log.Println("main: Initializing API support")

	// Make a channel to listen for an interrupt or terminate signal from OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	api := http.Server{
		Addr: cfg.Web.APIHost,
		//Handler: handlers.API(build, shutdown, log, db, auth),
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
	}

	serverErros := make(chan error, 1)

	go func() {
		log.Printf("main: API listening on %s", api.Addr)
		serverErros <- api.ListenAndServe()
	}()

	//===================================================================
	// Shutdown

	select {
	case err := <-serverErros:
		return errors.Wrap(err, "server error")
	case sig := <-shutdown:
		log.Printf("main: %v signal got : Start shutdown", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Asking listener to shutdown and shed load.
		if err := api.Shutdown(ctx); err != nil {
			api.Close()
			return errors.Wrap(err, "could not stop server gracefully")
		}
	}

	return nil
}
