package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

// ReadJSON - read json file to struct
func ReadJSON(fn string, v interface{}) error {
	file, err := os.Open(fn)
	defer file.Close()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)
	return decoder.Decode(v)
}

// HasPrefix tests case insensitive whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.ToLower(s[0:len(prefix)]) == strings.ToLower(prefix)
}

// Dumper - interface for dump data
type Dumper interface {
	Dump(params string, data string) error
}

// FileDumper - dumps data to file system
type FileDumper struct {
	Path    string
	DumpNum int
}

// Dump - dumps data to files
func (d *FileDumper) Dump(params string, data string) error {
	if _, err := os.Stat(d.Path); os.IsNotExist(err) {
		os.Mkdir(d.Path, 644)
	}
	d.DumpNum++
	err := ioutil.WriteFile(path.Join(d.Path, "dump"+strconv.Itoa(d.DumpNum)+".dmp"), []byte(params+"\n"+data), 0644)
	return err
}
