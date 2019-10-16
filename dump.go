package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const defaultDumpCheckInterval = 30

// ErrNoDumps - signal that dumps not found
var ErrNoDumps = errors.New("No dumps")

// Dumper - interface for dump data
type Dumper interface {
	Dump(params string, data string) error
}

// FileDumper - dumps data to file system
type FileDumper struct {
	Path        string
	DumpPrefix  string
	DumpNum     int
	LockedFiles map[string]bool
	mu          sync.Mutex
}

func (d *FileDumper) makePath(id string) string {
	return path.Join(d.Path, id)
}

func (d *FileDumper) checkDir(create bool) error {
	_, err := os.Stat(d.Path)
	if os.IsNotExist(err) {
		if create {
			return os.Mkdir(d.Path, 0777)
		}
	}
	return err
}

func (d *FileDumper) dumpName(num int) string {
	return "dump" + d.DumpPrefix + "-" + strconv.Itoa(num) + ".dmp"
}

// NewDumper - create new dumper
func NewDumper(path string) *FileDumper {
	d := new(FileDumper)
	d.Path = path
	d.DumpPrefix = time.Now().Format("20060102150405")
	return d
}

// Dump - dumps data to files
func (d *FileDumper) Dump(params string, data string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.checkDir(true)
	if err != nil {
		return err
	}
	d.DumpNum++
	err = ioutil.WriteFile(
		path.Join(d.Path, d.dumpName(d.DumpNum)), []byte(params+"\n"+data), 0644,
	)
	if err != nil {
		log.Printf("ERROR: dump to file: %+v\n", err)
	}
	return err
}

// GetDump - get dump file from filesystem
func (d *FileDumper) GetDump() (string, error) {
	err := d.checkDir(false)
	if os.IsNotExist(err) {
		return "", ErrNoDumps
	}
	if err != nil {
		return "", err
	}

	files, err := ioutil.ReadDir(d.Path)
	if err != nil {
		log.Fatal(err)
	}
	dumpFiles := make([]string, 0)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".dmp" {
			dumpFiles = append(dumpFiles, f.Name())
		}
	}

	queuedDumps.Set(float64(len(dumpFiles)))

	for _, f := range dumpFiles {
		found, _ := d.LockedFiles[f]
		if !found {
			return f, err
		}
	}
	return "", ErrNoDumps
}

// GetDumpData - get dump data from filesystem
func (d *FileDumper) GetDumpData(id string) (string, error) {
	path := d.makePath(id)
	s, err := ioutil.ReadFile(path)
	return string(s), err
}

// DeleteDump - get dump data from filesystem
func (d *FileDumper) DeleteDump(id string) error {
	path := d.makePath(id)
	err := os.Remove(path)
	return err
}

// ProcessNextDump - try to send next dump to server
func (d *FileDumper) ProcessNextDump(sender Sender) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	f, err := d.GetDump()
	if errors.Is(err, ErrNoDumps) {
		return err
	}
	if err != nil {
		return fmt.Errorf("Dump search error: %+v", err)
	}
	if f == "" {
		return nil
	}
	data, err := d.GetDumpData(f)
	if err != nil {
		return fmt.Errorf("Dump read error: %+v", err)
	}
	_, status, err := sender.SendQuery(data, "")
	if err != nil {
		return fmt.Errorf("server error (%+v) %+v", status, err)
	}
	log.Printf("INFO: dump sended: %+v\n", f)
	err = d.DeleteDump(f)
	if err != nil {
		d.LockedFiles[f] = true
		return fmt.Errorf("Dump delete error: %+v", err)
	}
	return err
}

// Listen - reads dumps from disk and try to send it
func (d *FileDumper) Listen(sender Sender, interval int) {
	d.LockedFiles = make(map[string]bool)
	if interval == 0 {
		interval = defaultDumpCheckInterval
	}
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	go func() {
		for range ticker.C {
			for {
				err := d.ProcessNextDump(sender)
				if err != nil {
					if !errors.Is(err, ErrNoDumps) {
						log.Printf("ERROR: %+v\n", err)
					}
					break
				}
			}
		}
	}()
}
