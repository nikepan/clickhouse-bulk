package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/labstack/echo"
)

// Server - main server object
type Server struct {
	Listen    string
	Collector *Collector
	Debug     bool
	echo      *echo.Echo
}

// Status - response status struct
type Status struct {
	Status    string                       `json:"status"`
	SendQueue int                          `json:"send_queue,omitempty"`
	Servers   map[string]*ClickhouseServer `json:"servers,omitempty"`
	Tables    map[string]*Table            `json:"tables,omitempty"`
}

// NewServer - create server
func NewServer(listen string, collector *Collector, debug bool) *Server {
	return &Server{listen, collector, debug, echo.New()}
}

func (server *Server) writeHandler(c echo.Context) error {
	q, _ := ioutil.ReadAll(c.Request().Body)
	s := string(q)

	if server.Debug {
		log.Printf("query %+v %+v\n", c.QueryString(), s)
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
		go server.Collector.Push(params, content)
		return c.String(http.StatusOK, "")
	}
	resp, status := server.Collector.Sender.SendQuery(params, content)
	return c.String(status, resp)
}

func (server *Server) statusHandler(c echo.Context) error {
	return c.JSON(200, Status{Status: "ok"})
}

// Start - start http server
func (server *Server) Start() error {
	return server.echo.Start(server.Listen)
}

// Shutdown - stop http server
func (server *Server) Shutdown(ctx context.Context) error {
	return server.echo.Shutdown(ctx)
}

// InitServer - run server
func InitServer(listen string, collector *Collector, debug bool) *Server {
	server := NewServer(listen, collector, debug)
	server.echo.POST("/", server.writeHandler)
	server.echo.GET("/status", server.statusHandler)

	return server
}
