package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"monitor/api"
	"monitor/lib"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"
)

type App struct {
	Port     string
	Database *sql.DB
	T        *template.Template
}

func (a App) Start(no_cron bool) {
	// migrate
	migrate_error := lib.AutoMigrate(a.Database)

	if migrate_error != nil {
		log.Fatalf("Error in AutoMigrate: %s", migrate_error)
	}
	// cron
	c := cron.New()

	c.AddFunc("@every 10m", func() {
		lib.Update(a.Database)
	})

	if !no_cron {
		c.Start()
	} else {
		log.Println("Skipping cron.Start() due to getting --offline flag")
	}

	// web
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.Handle("/api/servers/", lib.LogReq(a.ApiServers))
	http.Handle("/api/uptime/", lib.LogReq(a.ApiUptimes))
	http.Handle("/api/logs/", lib.LogReq(a.ApiLogs))
	http.Handle("/api/statuses/", lib.LogReq(a.ApiStatuses))
	http.Handle("/api/", lib.LogReq(a.Api))
	http.Handle("/export/", lib.LogReq(a.Export))
	http.Handle("/about/", lib.LogReq(a.About))
	http.Handle("/static/", lib.LogReq(lib.StaticHandler("static")))
	http.Handle("/metrics/", promhttp.Handler())
	http.Handle("/statuses/", lib.LogReq(a.Statuses))

	http.Handle("/", lib.LogReq(a.Index))

	addr := fmt.Sprintf(":%s", a.Port)

	log.Printf("Starting app on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func (a App) About(w http.ResponseWriter, r *http.Request) {
	lib.RenderTemplate(w, "about.html", nil)
}

func (a App) Api(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := struct {
		Routes []string `json:"routes"`
	}{
		Routes: []string{"/api/servers", "/api/uptime/:id", "/api/statuses/:id", "/api/logs"},
	}

	output, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		log.Fatal(err)
	}

	w.Write(output)
}

func (a App) Export(w http.ResponseWriter, r *http.Request) {
	x, err := ioutil.ReadFile(lib.Env("DB_PATH", "./monitor.db"))

	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(x)))
	w.Header().Set("Content-Disposition", "attachment; filename=\"monitor.sqlite3\"")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(http.StatusOK)
	w.Write(x)
}

func (a App) ApiServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var data []api.ServerAPIResponse = api.Servers(a.Database)

	output, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		log.Fatal(err)
	}

	w.Write(output)
}

func (a App) ApiUptimes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Pull out server id from URL
	// TODO: Put into subroutine
	re := regexp.MustCompile(`\/api\/uptime\/(\d+)`)
	m := re.FindStringSubmatch(r.URL.Path)

	if len(m) != 2 {
		log.Printf("Failed to extract server_id from %s. Returning HTTP 400.", r.URL.Path)

		w.WriteHeader(400)

		return
	}

	server_id, err := strconv.Atoi(m[1])

	if err != nil {
		log.Printf("Failed to convert %s to an int. Returning HTTP 500.", m[1])

		w.WriteHeader(500)

		return
	}

	var data []api.UptimeApiItem = api.Uptime(a.Database, server_id)

	output, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		log.Fatal(err)
	}

	w.Write(output)
}

func (a App) ApiLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var data []api.LogApiItem = api.Logs(a.Database)

	output, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		log.Fatal(err)
	}

	w.Write(output)
}

func (a App) ApiStatuses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	re := regexp.MustCompile(`\/api\/statuses\/(\d+)`)
	m := re.FindStringSubmatch(r.URL.Path)

	if len(m) != 2 {
		log.Printf("Failed to extract server_id from %s. Returning HTTP 400.", r.URL.Path)

		w.WriteHeader(400)

		return
	}

	server_id, err := strconv.Atoi(m[1])

	if err != nil {
		log.Printf("Failed to convert %s to an int. Returning HTTP 500.", m[1])

		w.WriteHeader(500)

		return
	}

	var data api.StatusApiResponse = api.Statuses(a.Database, server_id)

	output, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		log.Fatal(err)
	}

	w.Write(output)
}

func (a App) Statuses(w http.ResponseWriter, r *http.Request) {
	// Pull out server id from URL
	re := regexp.MustCompile(`\/statuses\/(\d+)`)
	m := re.FindStringSubmatch(r.URL.Path)

	if len(m) != 2 {
		log.Printf("Failed to extract server_id from %s. Returning HTTP 400.", r.URL.Path)
		w.WriteHeader(400)

		return
	}

	server_id, err := strconv.Atoi(m[1])

	if err != nil {
		log.Printf("Failed to convert %s to an int. Returning HTTP 500.", m[1])

		w.WriteHeader(500)

		return
	}

	var statuses api.StatusApiResponse = api.Statuses(a.Database, server_id)

	data := struct {
		Statuses api.StatusApiResponse
	}{
		Statuses: statuses,
	}

	lib.RenderTemplate(w, "statuses.html", data)
}

func (a App) Index(w http.ResponseWriter, r *http.Request) {
	var servers []api.ServerAPIResponseWithUptime = api.ServersWithUptimes(a.Database)
	var last_updated = lib.QueryLastUpdated(a.Database)

	data := struct {
		Servers     []api.ServerAPIResponseWithUptime
		LastUpdated string
	}{
		Servers:     servers,
		LastUpdated: last_updated,
	}

	lib.RenderTemplate(w, "index.html", data)
}

func main() {
	// Sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
                TracesSampleRate: 0.2,
	})

	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	defer sentry.Flush(2 * time.Second)

	// Logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// DB
	database, err := sql.Open("sqlite3", lib.Env("DB_PATH", "./monitor.db"))

	if err != nil {
		log.Fatal(err)
	}

	defer database.Close()

	// Serve (default) or handle args
	args := os.Args[1:]

	if len(args) == 1 && args[0] == "update" {
		lib.Update(database)

		return
	}

	// (Hackily) handle a --no-cron command line arg so we can start the
	// app with server fetchiing and checking off.
	no_cron := false

	if len(args) == 1 && args[0] == "--no-cron" {
		no_cron = true
	}

	// Prometheus
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	// Serve
	app := App{
		Port:     lib.Env("PORT", "8080"),
		Database: database,
	}

	app.Start(no_cron)
}
