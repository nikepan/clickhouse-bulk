package main

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"
)

// NoDumps - signal that dumps not found
type NoDumps struct {
}

func (e NoDumps) Error() string {
	return "No dumps"
}

// Dumper - interface for dump data
type Dumper interface {
	Dump(params string, data string) error
}

// FileDumper - dumps data to file system
type FileDumper struct {
	Path        string
	DumpNum     int
	LockedFiles map[string]bool
}

func (d *FileDumper) makePath(id string) string {
	return path.Join(d.Path, id)
}

func (d *FileDumper) checkDir() error {
	_, err := os.Stat(d.Path)
	if os.IsNotExist(err) {
		return os.Mkdir(d.Path, 777)
	}
	return err
}

// Dump - dumps data to files
func (d *FileDumper) Dump(params string, data string) error {
	err := d.checkDir()
	if err != nil {
		return err
	}
	d.DumpNum++
	err = ioutil.WriteFile(path.Join(d.Path, "dump"+strconv.Itoa(d.DumpNum)+".dmp"), []byte(params+"\n"+data), 0644)
	if err != nil {
		log.Printf("dump error: %+v\n", err)
	}
	return err
}

// GetDump - get dump file from filesystem
func (d *FileDumper) GetDump() (string, error) {
	err := d.checkDir()
	if err != nil {
		return "", err
	}

	files, err := ioutil.ReadDir(d.Path)
	if err != nil {
		log.Fatal(err)
	}
	dumpFiles := make([]string, 10)
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
	return "", &NoDumps{}
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
	f, err := d.GetDump()
	if errors.Is(err, NoDumps{}) {
		return err
	}
	if err != nil {
		log.Printf("dump search error: %+v\n", err)
	}
	if f == "" {
		return nil
	}
	data, err := d.GetDumpData(f)
	if err != nil {
		log.Printf("dump read error: %+v\n", err)
	}
	_, status := sender.SendQuery(data, "")
	if status < 300 {
		err := d.DeleteDump(f)
		if err != nil {
			d.LockedFiles[f] = true
			log.Printf("dump delete error: %+v\n", err)
		}
	}
	return err
}

// Listen - reads dumps from disk and try to send it
func (d *FileDumper) Listen(sender Sender, interval int) {
	d.LockedFiles = make(map[string]bool)
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	go func() {
		for range ticker.C {
			for {
				err := d.ProcessNextDump(sender)
				if err != nil {
					break
				}
			}
		}
	}()
}
