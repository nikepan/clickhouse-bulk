package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	// debug stuff
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server - main server object
type Server struct {
	Listen       string
	Collector    *Collector
	LiveSender   *Clickhouse
	LiveDumper   *FileDumper
	BackupSender *Clickhouse
	BackupDumper *FileDumper
	BackupOn     bool
	Debug        bool
	LogQueries   bool
	echo         *echo.Echo
}

// ReplayFailedResponse is returned by POST /debug/replay-failed.
type ReplayFailedResponse struct {
	Status string             `json:"status"`
	Live   *FailedReplayReport `json:"live,omitempty"`
	Backup *FailedReplayReport `json:"backup,omitempty"`
}

// NewServer - create server
func NewServer(listen string, collector *Collector, live *Clickhouse, backup *Clickhouse, backupOn bool, debug bool, logQueries bool) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	return &Server{
		Listen:       listen,
		Collector:    collector,
		LiveSender:   live,
		BackupSender: backup,
		BackupOn:     backupOn,
		Debug:        debug,
		LogQueries:   logQueries,
		echo:         e,
	}
}

func (server *Server) writeHandler(c echo.Context) error {
	q, _ := io.ReadAll(c.Request().Body)
	s := string(q)

	if server.Debug {
		log.Printf("DEBUG: query %+v %+v\n", c.QueryString(), s)
	}

	qs := c.QueryString()
	user, password, ok := c.Request().BasicAuth()
	if ok {
		if qs == "" {
			qs = "user=" + user + "&password=" + password
		} else {
			qs = "user=" + user + "&password=" + password + "&" + qs
		}
	}
	params, content, insert := server.Collector.ParseQuery(qs, s)
	if insert {
		if len(content) == 0 {
			log.Printf("INFO: empty insert params: [%+v] content: [%+v]\n", params, content)
			return c.String(http.StatusInternalServerError, "Empty insert\n")
		}
		var journalID uint64
		if server.Collector.Journal != nil {
			id, err := server.Collector.Journal.Append(params, content)
			if err != nil {
				log.Printf("ERROR: journal append: %+v\n", err)
				if errors.Is(err, ErrJournalBacklog) {
					return c.String(http.StatusServiceUnavailable, "Journal backlog full\n")
				}
				return c.String(http.StatusInternalServerError, "Journal write failed\n")
			}
			journalID = id
			setJournalPendingGauge(server.Collector.Journal)
		}
		go server.Collector.Push(params, content, journalID)
		return c.String(http.StatusOK, "")
	}
	resp, status, _ := server.Collector.Sender.SendQuery(&ClickhouseRequest{Params: qs, Content: s, isInsert: false})
	return c.String(status, resp)
}

func (server *Server) statusHandler(c echo.Context) error {
	st := FullStatus{
		Status: "ok",
		Live:   buildTargetStatus(server.LiveSender, server.LiveSender != nil),
		Backup: buildTargetStatus(server.BackupSender, server.BackupOn),
	}
	return c.JSON(200, st)
}

func (server *Server) gcHandler(c echo.Context) error {
	runtime.GC()
	return c.JSON(200, map[string]string{"status": "GC"})
}

func (server *Server) freeMemHandler(c echo.Context) error {
	debug.FreeOSMemory()
	return c.JSON(200, map[string]string{"status": "freeMem"})
}

// manual trigger for cleaning tables
func (server *Server) tablesCleanHandler(c echo.Context) error {
	server.Collector.mu.Lock()
	var remove []string
	for k, t := range server.Collector.Tables {
		if t.Empty() {
			t.CleanTable()
			remove = append(remove, k)
		}
	}
	for _, k := range remove {
		delete(server.Collector.Tables, k)
	}
	server.Collector.mu.Unlock()
	return c.JSON(200, map[string]string{"status": "cleaned empty tables"})
}

// replayFailedHandler replays .dmp files from dump_dir/failed/ (ex-4xx batches).
// Query: target=live|backup|all (default live), limit=N (0 = all files).
func (server *Server) replayFailedHandler(c echo.Context) error {
	target := c.QueryParam("target")
	if target == "" {
		target = "live"
	}
	limit := 0
	if s := c.QueryParam("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid limit"})
		}
		limit = v
	}
	resp := ReplayFailedResponse{Status: "ok"}
	switch target {
	case "backup":
		if !server.BackupOn || server.BackupDumper == nil || server.BackupSender == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not enabled"})
		}
		r := server.BackupDumper.ReplayFailed(server.BackupSender, limit)
		resp.Backup = &r
	case "all":
		if server.LiveDumper != nil && server.LiveSender != nil {
			r := server.LiveDumper.ReplayFailed(server.LiveSender, limit)
			resp.Live = &r
		}
		if server.BackupOn && server.BackupDumper != nil && server.BackupSender != nil {
			r := server.BackupDumper.ReplayFailed(server.BackupSender, limit)
			resp.Backup = &r
		}
	default:
		if server.LiveDumper == nil || server.LiveSender == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "live dumper not configured"})
		}
		r := server.LiveDumper.ReplayFailed(server.LiveSender, limit)
		resp.Live = &r
	}
	return c.JSON(http.StatusOK, resp)
}

// Start - start http server
func (server *Server) Start(cnf Config) error {
	if cnf.UseTLS {
		return server.echo.StartTLS(server.Listen, cnf.TLSCertFile, cnf.TLSKeyFile)
	} else {
		return server.echo.Start(server.Listen)
	}
}

// Shutdown - stop http server
func (server *Server) Shutdown(ctx context.Context) error {
	return server.echo.Shutdown(ctx)
}

// InitServer - run server
func InitServer(listen string, collector *Collector, live *Clickhouse, liveDumper *FileDumper, backup *Clickhouse, backupDumper *FileDumper, backupOn bool, debug bool, logQueries bool) *Server {
	server := NewServer(listen, collector, live, backup, backupOn, debug, logQueries)
	server.LiveDumper = liveDumper
	server.BackupDumper = backupDumper
	server.echo.POST("/", server.writeHandler)
	server.echo.GET("/status", server.statusHandler)
	server.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	// debug / ops
	server.echo.POST("/debug/replay-failed", server.replayFailedHandler)
	server.echo.GET("/debug/replay-failed", server.replayFailedHandler)
	server.echo.GET("/debug/gc", server.gcHandler)
	server.echo.GET("/debug/freemem", server.freeMemHandler)
	server.echo.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	server.echo.GET("/debug/tables-clean", server.tablesCleanHandler)

	return server
}

// SafeQuit flushes in-memory batches and waits for sender queues to drain.
func SafeQuit(collect *Collector, sender Sender, drainSec int) {
	if drainSec <= 0 {
		drainSec = 60
	}
	deadline := time.Now().Add(time.Duration(drainSec) * time.Second)
	collect.FlushAll()
	for !sender.Empty() || !collect.Empty() {
		if time.Now().After(deadline) {
			log.Printf("WARN: shutdown drain timeout after %ds (queue=%+v collector_empty=%+v)\n",
				drainSec, sender.Len(), collect.Empty())
			return
		}
		if !collect.Empty() {
			collect.FlushAll()
		}
		if count := sender.Len(); count > 0 {
			log.Printf("Draining send queue (%+v items pending)\n", count)
		}
		collect.WaitFlush()
	}
}

func newClickhouseSender(ch clickhouseConfig, dumpDir string, logQueries bool, metricTarget string, maxDumpFiles int) (*Clickhouse, *FileDumper) {
	dumper := NewDumper(dumpDir)
	dumper.MetricTarget = metricTarget
	dumper.MaxDumpFiles = maxDumpFiles
	sender := NewClickhouse(ch.DownTimeout, ch.ConnectTimeout, ch.TLSServerName, ch.TLSSkipVerify, ch.SendMaxRPS, ch.SendMaxBurst)
	sender.MetricTarget = metricTarget
	if ch.SendMaxRPS > 0 {
		log.Printf("INFO: %s send rate limit: %d rps (burst %d)\n", metricTarget, ch.SendMaxRPS, ch.SendMaxBurst)
	}
	sender.QueryParams = ch.QueryParams
	sender.Dumper = dumper
	for _, url := range ch.Servers {
		sender.AddServer(url, logQueries)
	}
	return sender, dumper
}

func startDumpReplay(dumper *FileDumper, sender Sender, interval int, replayBatch int) {
	if interval >= 0 {
		dumper.Listen(sender, interval, replayBatch)
	}
}

func backupDumpCheckInterval(cnf Config) int {
	if cnf.BkpDumpCheckInterval > 0 {
		return cnf.BkpDumpCheckInterval
	}
	return cnf.DumpCheckInterval
}

// RunServer - run all
func RunServer(cnf Config) {
	InitMetrics(cnf.MetricsPrefix, cnf.BackupEnabled())

	journal, err := NewJournal(cnf.JournalDir, cnf.JournalFsync, cnf.MaxJournalPending)
	if err != nil {
		log.Fatalf("ERROR: journal: %+v\n", err)
	}
	defer journal.Close()

	liveSender, liveDumper := newClickhouseSender(cnf.Clickhouse, cnf.DumpDir, cnf.LogQueries, "", cnf.MaxDumpFiles)
	liveSender.Journal = journal
	var sender Sender = liveSender
	var backupSender *Clickhouse
	var backupDumper *FileDumper

	if cnf.BackupEnabled() {
		bkpDumpDir := cnf.BkpDumpDir
		if bkpDumpDir == "" {
			bkpDumpDir = "dumps-bkp"
		}
		backupSender, backupDumper = newClickhouseSender(*cnf.ClickhouseBackup, bkpDumpDir, cnf.LogQueries, metricTargetBackup, cnf.MaxDumpFiles)
		sender = NewDualSender(liveSender, backupSender)
		startDumpReplay(liveDumper, liveSender, cnf.DumpCheckInterval, cnf.DumpReplayBatch)
		startDumpReplay(backupDumper, backupSender, backupDumpCheckInterval(cnf), cnf.DumpReplayBatch)
		log.Printf("Backup mode: live servers %+v, backup servers %+v, bkp_dump_dir=%s, bkp_dump_check_interval=%ds",
			cnf.Clickhouse.Servers, cnf.ClickhouseBackup.Servers, bkpDumpDir, backupDumpCheckInterval(cnf))
	} else {
		startDumpReplay(liveDumper, liveSender, cnf.DumpCheckInterval, cnf.DumpReplayBatch)
	}

	collect := NewCollector(sender, journal, cnf.FlushCount, cnf.FlushInterval, cnf.CleanInterval, cnf.RemoveQueryID)

	if journal != nil {
		RegisterJournalMetrics()
		if err := journal.ReplayUnacked(collect.Push); err != nil {
			log.Fatalf("ERROR: journal replay: %+v\n", err)
		}
		if err := journal.Compact(); err != nil {
			log.Printf("WARN: journal compact: %+v\n", err)
		}
		setJournalPendingGauge(journal)
		log.Printf("Journal enabled: dir=%s fsync=%v\n", cnf.JournalDir, cnf.JournalFsync)
	}

	// send collected data on SIGTERM and exit
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	srv := InitServer(cnf.Listen, collect, liveSender, liveDumper, backupSender, backupDumper, cnf.BackupEnabled(), cnf.Debug, cnf.LogQueries)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		<-signals
		log.Printf("STOP signal received, shutting down\n")
		exitCode := 0
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown error: %+v\n", err)
			exitCode = 1
		}
		SafeQuit(collect, sender, cnf.ShutdownDrainSec)
		log.Printf("Shutdown complete, exiting\n")
		os.Exit(exitCode)
	}()

	log.Printf("Server starting on %s\n", cnf.Listen)
	err = srv.Start(cnf)
	if err != nil && err != http.ErrServerClosed {
		log.Printf("ListenAndServe: %+v\n", err)
		SafeQuit(collect, sender, cnf.ShutdownDrainSec)
		os.Exit(1)
	}
}
