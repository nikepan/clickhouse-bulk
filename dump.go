package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultDumpCheckInterval = 30
const dumpResponseMark = "\n### RESPONSE ###\n"

// ErrNoDumps - signal that dumps not found
var ErrNoDumps = errors.New("No dumps")

// Dumper - interface for dump data
type Dumper interface {
	Dump(params string, data string, response string, prefix string, status int) error
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
			return os.Mkdir(d.Path, 0666)
		}
	}
	return err
}

func (d *FileDumper) dumpName(num int, prefix string, status int) string {
	return "dump" + d.DumpPrefix + prefix + "-" + strconv.Itoa(num) + "-" + strconv.Itoa(status) + ".dmp"
}

// NewDumper - create new dumper
func NewDumper(path string) *FileDumper {
	d := new(FileDumper)
	d.Path = path
	d.DumpPrefix = time.Now().Format("20060102150405")
	return d
}

// Dump - dumps data to files
func (d *FileDumper) Dump(params string, content string, response string, prefix string, status int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.checkDir(true)
	if err != nil {
		return err
	}
	data := params + "\n" + content
	if response != "" {
		data += dumpResponseMark + response
	}
	d.DumpNum++
	file_path := path.Join(d.Path, d.dumpName(d.DumpNum, prefix, status))
	err = os.WriteFile(file_path, []byte(data), 0644)
	if err != nil {
		log.Printf("ERROR: dump to file: %+v\n", err)
	} else {
		log.Printf("SUCCESS: dump to file: %+v\n", file_path)
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

	files, err := os.ReadDir(d.Path)
	if err != nil {
		return "", err
	}
	dumpFiles := make([]string, 0)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".dmp" {
			dumpFiles = append(dumpFiles, f.Name())
		}
	}
	sort.Strings(dumpFiles)

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
func (d *FileDumper) GetDumpData(id string) (data string, response string, err error) {
	path := d.makePath(id)
	s, err := os.ReadFile(path)
	items := strings.Split(string(s), dumpResponseMark)
	if len(items) > 1 {
		return items[0], items[1], err
	}
	return items[0], "", err
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
	data, _, err := d.GetDumpData(f)
	if err != nil {
		return fmt.Errorf("Dump read error: %+v", err)
	}
	if data != "" {
		params := ""
		query := ""
		lines := strings.Split(data, "\n")
		if !HasPrefix(lines[0], "insert") {
			params = lines[0]
			query = lines[1]
			data = strings.Join(lines[1:], "\n")
		}
		_, status, err := sender.SendQuery(&ClickhouseRequest{Params: params, Content: data, Query: query, Count: len(lines[2:]), isInsert: true})
		if err != nil {
			return fmt.Errorf("server error (%+v) %+v", status, err)
		}
		log.Printf("INFO: dump sent: %+v\n", f)
	}
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
