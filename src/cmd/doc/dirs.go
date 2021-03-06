// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/build"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Dirs is a structure for scanning the directory tree.
// Its Next method returns the next Go source directory it finds.
// Although it can be used to scan the tree multiple times, it
// only walks the tree once, caching the data it finds.
type Dirs struct {
	scan   chan string // directories generated by walk.
	paths  []string    // Cache of known paths.
	offset int         // Counter for Next.
}

var dirs Dirs

func init() {
	dirs.paths = make([]string, 0, 1000)
	dirs.scan = make(chan string)
	go dirs.walk()
}

// Reset puts the scan back at the beginning.
func (d *Dirs) Reset() {
	d.offset = 0
}

// Next returns the next directory in the scan. The boolean
// is false when the scan is done.
func (d *Dirs) Next() (string, bool) {
	if d.offset < len(d.paths) {
		path := d.paths[d.offset]
		d.offset++
		return path, true
	}
	path, ok := <-d.scan
	if !ok {
		return "", false
	}
	d.paths = append(d.paths, path)
	d.offset++
	return path, ok
}

// walk walks the trees in GOROOT and GOPATH.
func (d *Dirs) walk() {
	d.bfsWalkRoot(build.Default.GOROOT)
	for _, root := range splitGopath() {
		d.bfsWalkRoot(root)
	}
	close(d.scan)
}

// bfsWalkRoot walks a single directory hierarchy in breadth-first lexical order.
// Each Go source directory it finds is delivered on d.scan.
func (d *Dirs) bfsWalkRoot(root string) {
	root = path.Join(root, "src")

	// this is the queue of directories to examine in this pass.
	this := []string{}
	// next is the queue of directories to examine in the next pass.
	next := []string{root}

	for len(next) > 0 {
		this, next = next, this[0:0]
		for _, dir := range this {
			fd, err := os.Open(dir)
			if err != nil {
				log.Print(err)
				continue
			}
			entries, err := fd.Readdir(0)
			fd.Close()
			if err != nil {
				log.Print(err)
				continue
			}
			hasGoFiles := false
			for _, entry := range entries {
				name := entry.Name()
				// For plain files, remember if this directory contains any .go
				// source files, but ignore them otherwise.
				if !entry.IsDir() {
					if !hasGoFiles && strings.HasSuffix(name, ".go") {
						hasGoFiles = true
					}
					continue
				}
				// Entry is a directory.
				// No .git or other dot nonsense please.
				if strings.HasPrefix(name, ".") {
					continue
				}
				// Remember this (fully qualified) directory for the next pass.
				next = append(next, filepath.Join(dir, name))
			}
			if hasGoFiles {
				// It's a candidate.
				d.scan <- dir
			}
		}

	}
}
