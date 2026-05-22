package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultDumpCheckInterval = 30
const dumpResponseMark = "\n### RESPONSE ###\n"
const dumpKindTransient = "1"
const dumpKindClientError = "2"
const failedDumpSubdir = "failed"

var clientErrorDumpPattern = regexp.MustCompile(`^dump\d{14}` + dumpKindClientError + `-\d+-\d+\.dmp$`)

// ErrNoDumps - signal that dumps not found
var ErrNoDumps = errors.New("No dumps")

// ErrInvalidDumpID is returned when a dump file id contains path traversal or invalid components.
var ErrInvalidDumpID = errors.New("invalid dump file id")

// Dumper - interface for dump data
type Dumper interface {
	Dump(params string, data string, response string, prefix string, status int) error
}

// FileDumper - dumps data to file system
type FileDumper struct {
	Path         string
	DumpPrefix   string
	DumpNum      int
	MaxDumpFiles int
	MetricTarget string
	LockedFiles  map[string]bool
	mu           sync.Mutex
}

// safeDumpRelPath validates dump file ids from disk listings or replay (basename or failed/<basename> only).
func safeDumpRelPath(id string) (string, error) {
	if strings.Contains(filepath.ToSlash(id), "..") {
		return "", ErrInvalidDumpID
	}
	clean := filepath.Clean(id)
	slash := filepath.ToSlash(clean)
	if slash == failedDumpSubdir {
		return "", ErrInvalidDumpID
	}
	if strings.HasPrefix(slash, failedDumpSubdir+"/") {
		base := filepath.Base(clean)
		if base == "" || base == "." || base == ".." {
			return "", ErrInvalidDumpID
		}
		return path.Join(failedDumpSubdir, base), nil
	}
	if strings.Contains(slash, "/") {
		return "", ErrInvalidDumpID
	}
	base := filepath.Base(clean)
	if base == "" || base == "." || base == ".." {
		return "", ErrInvalidDumpID
	}
	return base, nil
}

func (d *FileDumper) makePath(id string) (string, error) {
	rel, err := safeDumpRelPath(id)
	if err != nil {
		return "", err
	}
	full := path.Join(d.Path, rel)
	// Ensure resolved path stays under dump root.
	if relCheck, err := filepath.Rel(d.Path, full); err != nil || strings.HasPrefix(relCheck, "..") || relCheck == ".." {
		return "", ErrInvalidDumpID
	}
	return full, nil
}

func (d *FileDumper) checkDir(create bool) error {
	_, err := os.Stat(d.Path)
	if os.IsNotExist(err) {
		if create {
			return os.Mkdir(d.Path, 0766)
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
		d.pruneOldestIfNeeded()
		d.updateDirMetrics()
	}
	return err
}

func (d *FileDumper) listPendingDumpFiles() ([]string, error) {
	err := d.checkDir(false)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, f := range entries {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".dmp" {
			out = append(out, f.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (d *FileDumper) pruneOldestIfNeeded() {
	if d.MaxDumpFiles <= 0 {
		return
	}
	files, err := d.listPendingDumpFiles()
	if err != nil {
		return
	}
	for len(files) > d.MaxDumpFiles {
		oldest := files[0]
		p, err := d.makePath(oldest)
		if err != nil {
			break
		}
		if err := os.Remove(p); err != nil {
			log.Printf("ERROR: prune dump %+v: %+v\n", oldest, err)
			break
		}
		log.Printf("WARN: pruned oldest dump (max_dump_files=%+v): %+v\n", d.MaxDumpFiles, oldest)
		files = files[1:]
	}
}

func (d *FileDumper) updateDirMetrics() {
	var total int64
	_ = filepath.Walk(d.Path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	setDumpDirBytesGauge(d.MetricTarget, total)
}

func (d *FileDumper) effectiveReplayBatch(configured int) int {
	if configured > 0 {
		return configured
	}
	return 0
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

	dumpFiles, err := d.listPendingDumpFiles()
	if err != nil {
		return "", err
	}

	setQueuedDumpsGauge(d.MetricTarget, len(dumpFiles))
	d.updateDirMetrics()

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
	filePath, err := d.makePath(id)
	if err != nil {
		return "", "", err
	}
	s, err := os.ReadFile(filePath)
	items := strings.Split(string(s), dumpResponseMark)
	if len(items) > 1 {
		return items[0], items[1], err
	}
	return items[0], "", err
}

func isClientErrorDumpFile(name string) bool {
	return clientErrorDumpPattern.MatchString(name)
}

func (d *FileDumper) failedDir() string {
	return path.Join(d.Path, failedDumpSubdir)
}

func (d *FileDumper) moveToFailed(name string) error {
	safeName, err := safeDumpRelPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.failedDir(), 0766); err != nil {
		return err
	}
	src, err := d.makePath(safeName)
	if err != nil {
		return err
	}
	dst := path.Join(d.failedDir(), safeName)
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	log.Printf("INFO: dump moved to failed (no retry): %s -> %s\n", src, dst)
	return nil
}

// DeleteDump - remove a dump file by id (basename or failed/<basename>).
func (d *FileDumper) DeleteDump(id string) error {
	filePath, err := d.makePath(id)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		err = os.Remove(filePath)
		if err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

func parseDumpPayload(data string) (params, query, content string, rowCount int) {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 {
		return "", "", "", 0
	}
	if HasPrefix(lines[0], "insert") {
		return "", lines[0], strings.Join(lines[1:], "\n"), len(lines) - 1
	}
	params = lines[0]
	if len(lines) > 1 {
		query = lines[1]
	}
	content = strings.Join(lines[1:], "\n")
	if len(lines) > 2 {
		rowCount = len(lines[2:])
	}
	return params, query, content, rowCount
}

func (d *FileDumper) sendDumpPayload(sender Sender, data string) (status int, err error) {
	params, query, content, rowCount := parseDumpPayload(data)
	if content == "" && query == "" && params == "" {
		return http.StatusOK, nil
	}
	_, st, sendErr := sender.SendQuery(&ClickhouseRequest{
		Params: params, Query: query, Content: content, Count: rowCount, isInsert: true,
	})
	return st, sendErr
}

func (d *FileDumper) listFailedDumpFiles() ([]string, error) {
	dir := d.failedDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".dmp" {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// FailedReplayItem is one file result from ReplayFailed.
type FailedReplayItem struct {
	File   string `json:"file"`
	Status int    `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// FailedReplayReport summarizes replay from failed/.
type FailedReplayReport struct {
	Sent      int                `json:"sent"`
	Errors    int                `json:"errors"`
	Remaining int                `json:"remaining"`
	Items     []FailedReplayItem `json:"items,omitempty"`
}

// ReplayFailed sends dumps from failed/ via sender (manual / HTTP trigger).
// limit is max files to attempt; 0 = all. Successful sends delete the file.
func (d *FileDumper) ReplayFailed(sender Sender, limit int) FailedReplayReport {
	report := FailedReplayReport{}
	d.mu.Lock()
	defer d.mu.Unlock()

	files, err := d.listFailedDumpFiles()
	if err != nil {
		report.Items = []FailedReplayItem{{File: "", Error: err.Error()}}
		report.Errors++
		return report
	}
	processed := 0
	for _, name := range files {
		if limit > 0 && processed >= limit {
			break
		}
		processed++
		item := FailedReplayItem{File: name}
		rel := path.Join(failedDumpSubdir, name)
		data, _, err := d.GetDumpData(rel)
		if err != nil {
			item.Error = err.Error()
			report.Errors++
			report.Items = append(report.Items, item)
			continue
		}
		status, err := d.sendDumpPayload(sender, data)
		if err != nil {
			item.Status = status
			item.Error = err.Error()
			report.Errors++
			report.Items = append(report.Items, item)
			log.Printf("ERROR: replay failed dump %s: (%+v) %+v\n", name, status, err)
			continue
		}
		rmPath, err := d.makePath(rel)
		if err != nil {
			item.Error = err.Error()
			report.Errors++
			report.Items = append(report.Items, item)
			continue
		}
		if err := os.Remove(rmPath); err != nil {
			item.Error = "sent but delete failed: " + err.Error()
			report.Errors++
			report.Items = append(report.Items, item)
			continue
		}
		report.Sent++
		report.Items = append(report.Items, item)
		log.Printf("INFO: replay failed dump sent: %s\n", name)
	}
	remaining, _ := d.listFailedDumpFiles()
	report.Remaining = len(remaining)
	d.updateDirMetrics()
	return report
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
	if isClientErrorDumpFile(f) {
		if err := d.moveToFailed(f); err != nil {
			return fmt.Errorf("move client-error dump to failed: %+v", err)
		}
		return nil
	}
	data, _, err := d.GetDumpData(f)
	if err != nil {
		return fmt.Errorf("Dump read error: %+v", err)
	}
	if data != "" {
		status, err := d.sendDumpPayload(sender, data)
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

// Listen reads dumps from disk and tries to send them on a schedule.
// replayBatch limits files processed per tick (0 = unlimited).
func (d *FileDumper) Listen(sender Sender, interval int, replayBatch int) {
	d.LockedFiles = make(map[string]bool)
	if interval == 0 {
		interval = defaultDumpCheckInterval
	}
	batchLimit := d.effectiveReplayBatch(replayBatch)
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	go func() {
		for range ticker.C {
			processed := 0
			for {
				if batchLimit > 0 && processed >= batchLimit {
					break
				}
				err := d.ProcessNextDump(sender)
				if err != nil {
					if !errors.Is(err, ErrNoDumps) {
						log.Printf("ERROR: %+v\n", err)
					}
					break
				}
				processed++
			}
			d.updateDirMetrics()
		}
	}()
}
