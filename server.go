package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	Listen     string
	Collector  *Collector
	Debug      bool
	LogQueries bool
	echo       *echo.Echo
}

// Status - response status struct
type Status struct {
	Status    string                       `json:"status"`
	SendQueue int                          `json:"send_queue,omitempty"`
	Servers   map[string]*ClickhouseServer `json:"servers,omitempty"`
	Tables    map[string]*Table            `json:"tables,omitempty"`
}

// NewServer - create server
func NewServer(listen string, collector *Collector, debug bool, logQueries bool) *Server {
	return &Server{listen, collector, debug, logQueries, echo.New()}
}

func (server *Server) writeHandler(c echo.Context) error {
	req := c.Request()
	q, _ := ioutil.ReadAll(req.Body)
	s := string(q)

	if server.Debug {
		log.Printf("DEBUG: query %+v %+v\n", c.QueryString(), s)
	}

	qs := c.QueryString()
	server.Collector.Sender.SetCreds(getAuth(req))

	params, content, insert := server.Collector.ParseQuery(qs, s)
	if insert {
		if len(content) == 0 {
			log.Printf("INFO: empty insert params: [%+v] content: [%+v]\n", params, content)
			return c.String(http.StatusInternalServerError, "Empty insert\n")
		}
		go server.Collector.Push(params, content)
		return c.String(http.StatusOK, "")
	}

	res, buf := server.Collector.Sender.PassThru(req, q)

	defer res.Body.Close()
	CopyHeader(c.Response().Header(), res.Header)
	c.Response().WriteHeader(res.StatusCode)
	c.Response().Header().Set("Collection", "close")
	return c.Stream(200, "application/octet-stream", buf)
}

func (server *Server) statusHandler(c echo.Context) error {
	return c.JSON(200, Status{Status: "ok"})
}

func (server *Server) gcHandler(c echo.Context) error {
	runtime.GC()
	return c.JSON(200, Status{Status: "GC"})
}

func (server *Server) freeMemHandler(c echo.Context) error {
	debug.FreeOSMemory()
	return c.JSON(200, Status{Status: "freeMem"})
}

// manual trigger for cleaning tables
func (server *Server) tablesCleanHandler(c echo.Context) error {
	log.Printf("DEBUG: clean tables:\n%+v", server.Collector.Tables)
	for k, t := range server.Collector.Tables {
		log.Printf("DEBUG: check if table is empty: %+v with key:%+v\n", t, k)
		if ok := t.Empty(); ok {
			log.Printf("DEBUG: delete empty table: %+v with key:%+v\n", t, k)
			server.Collector.Tables[k].CleanTable()
			defer delete(server.Collector.Tables, k)
		}
	}
	return c.JSON(200, Status{Status: "cleaned empty tables"})
}

// Start - start http server
func (server *Server) Start() error {
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
func InitServer(listen string, collector *Collector, debug bool, logQueries bool) *Server {
	server := NewServer(listen, collector, debug, logQueries)
	server.echo.POST("/", server.writeHandler)
	server.echo.GET("/status", server.statusHandler)
	server.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	// debug stuff
	server.echo.GET("/debug/gc", server.gcHandler)
	server.echo.GET("/debug/freemem", server.freeMemHandler)
	server.echo.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	server.echo.GET("/debug/tables-clean", server.tablesCleanHandler)

	return server
}

// SafeQuit - safe prepare to quit
func SafeQuit(collect *Collector, sender Sender) {
	collect.FlushAll()
	if count := sender.Len(); count > 0 {
		log.Printf("Sending %+v tables\n", count)
	}
	for !sender.Empty() && !collect.Empty() {
		collect.WaitFlush()
	}
	collect.WaitFlush()
}

// RunServer - run all
func RunServer() {
	InitMetrics(cnf.MetricsPrefix)
	dumper := NewDumper(cnf.DumpDir)
	sender := NewClickhouse(cnf.Clickhouse.DownTimeout, cnf.Clickhouse.ConnectTimeout, cnf.Clickhouse.tlsServerName, cnf.Clickhouse.tlsSkipVerify)
	sender.Dumper = dumper
	for _, url := range cnf.Clickhouse.Servers {
		sender.AddServer(url, cnf.LogQueries)
	}

	collect := NewCollector(sender, cnf.FlushCount, cnf.FlushInterval, cnf.CleanInterval, cnf.RemoveQueryID)

	// send collected data on SIGTERM and exit
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	srv := InitServer(cnf.Listen, collect, cnf.Debug, cnf.LogQueries)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		for {
			_ = <-signals
			log.Printf("STOP signal\n")
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("Shutdown error %+v\n", err)
				SafeQuit(collect, sender)
				os.Exit(1)
			}
		}
	}()

	if cnf.DumpCheckInterval >= 0 {
		dumper.Listen(sender, cnf.DumpCheckInterval)
	}

	err := srv.Start()
	if err != nil {
		log.Printf("ListenAndServe: %+v\n", err)
		SafeQuit(collect, sender)
		os.Exit(1)
	}
}
