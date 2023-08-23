// @title				Dele Movie API
// @version			1.0
// @description		A movie service API in Go using Gin framework.
// @tag.name			This is the name of the tag
// @tag.description	This is the description of the tag
// @termsOfService		http://swagger.io/terms/
// @contact.name		Ocean Baoscrip
// @contact.url		https://twitter.com/oceanbao
// @contact.email		ocean.hy.bao@gmail.com
// @license.name		Apache 2.0
// @license.url		http://www.apache.org/licenses/LICENSE-2.0.html
// @host				localhost:4000
// @BasePath			/v1
package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"

	// "log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"example.com/gofurther/internal/data"
	"example.com/gofurther/internal/jsonlog"
	"example.com/gofurther/internal/mailer"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// "burn-in" variable must be string, as -X linter flag requires.
var (
	buildTime string
	version   string
)

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	// logger *log.Logger
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	_ = godotenv.Load(".env")
	// if err != nil {
	// log.Fatalf("Cannot load .env file: %s", err)
	// }

	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("POSTGRES_DSN"), "PostgreSQL DSN")
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idel time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "ad13e86792e908", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "72b58f344ae90f", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.org>", "SMTP sender")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space delimited", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		fmt.Printf("Build time:\t%s\n", buildTime)
		os.Exit(0)
	}

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("database connection pool established", nil)

	// Metrics
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("database", expvar.Func(func() interface{} {
		return db.Stats()
	}))
	expvar.Publish("timestamp", expvar.Func(func() interface{} {
		return time.Now().Unix()
	}))

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
