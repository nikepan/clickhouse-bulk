package main

import (
	"github.com/labstack/echo"
	"io/ioutil"
	"log"
	"net/http"
)

// Server - main server object
type Server struct {
	Listen    string
	Collector *Collector
	Debug     bool
}

type Status struct {
	Status    string                       `json:"status"`
	SendQueue int                          `json:"send_queue,omitempty"`
	Servers   map[string]*ClickhouseServer `json:"servers,omitempty"`
	Tables    map[string]*Table            `json:"tables,omitempty"`
}

// NewServer - create server
func NewServer(listen string, collector *Collector, debug bool) *Server {
	return &Server{listen, collector, debug}
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
			qs = "user="+user+"&password="+password
		} else {
			qs = "user="+user+"&password="+password + "&" + qs
		}
	}
	params, content, insert := server.Collector.ParseQuery(qs, s)
	if insert {
		go server.Collector.Push(params, content)
		return c.String(http.StatusOK, "")
	} else {
		resp, status := server.Collector.Sender.SendQuery(params, content)
		return c.String(status, resp)
	}
}

func (server *Server) statusHandler(c echo.Context) error {
	return c.JSON(200, Status{Status: "ok"})
}

// RunServer - run server
func RunServer(listen string, collector *Collector, debug bool) error {
	server := NewServer(listen, collector, debug)
	e := echo.New()
	e.POST("/", server.writeHandler)
	e.GET("/status", server.statusHandler)

	return e.Start(server.Listen)
}
