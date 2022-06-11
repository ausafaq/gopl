package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var v = flag.Bool("v", false, "show verbose progress messages")
var tokens = make(chan struct{}, 20) // limiting concurrency to directory entries

// aggregated message
type Message struct {
	filesize int64
	id       int
}

func main() {
	flag.Parse()

	roots := flag.Args()
	size := len(roots)

	if size == 0 {
		roots = []string{"."}
		size = 1
	}

	var message = make(chan Message)
	var wg sync.WaitGroup

	var filesizes = make([]chan int64, size)
	var n = make([]*sync.WaitGroup, size)

	for i := 0; i < size; i++ {
		filesizes[i] = make(chan int64)
		n[i] = new(sync.WaitGroup)
	}

	for i, root := range roots {
		n[i].Add(i)

		go walkDir(root, n[i], filesizes[i])

		go func(i int) {
			n[i].Wait()
			close(filesizes[i])
		}(i) // eliminate loop variable capture

		wg.Add(1) // gather all filesize message with extra id infor
		go func(i int) {
			defer wg.Done()
			var msg Message
			for fs := range filesizes[i] {
				msg.filesize = fs
				msg.id = i
				message <- msg
			}
		}(i)
	}

	// message closer
	go func() {
		wg.Wait()
		close(message)
	}()

	// prints the results periodically
	ticker := time.NewTicker(1 * time.Second)

	nfiles := make([]int64, size)
	nbytes := make([]int64, size)

	var tfiles, tbytes int64 // total

loop:
	for {
		select {
		case msg, ok := <-message:
			if !ok {
				break loop
			}
			nfiles[msg.id]++
			nbytes[msg.id] += msg.filesize

			tfiles++
			tbytes += msg.filesize
		case <-ticker.C:
			printAllDiskUsage(roots, nfiles, nbytes)
		}
	}
	// ticker.Stop()
	printDiskUsage(tfiles, tbytes) // final totals
}

func printAllDiskUsage(file []string, nfiles, nbytes []int64) {
	for i := range nfiles {
		fmt.Printf("%s: %d files  %.1f GB; ", file[i], nfiles[i], float64(nbytes[i])/1e9)
	}
	fmt.Println()
}

func printDiskUsage(nfiles, nbytes int64) {
	fmt.Printf("%d files  %.1f GB\n", nfiles, float64(nbytes)/1e9)
}

// walkDir recursively walks the file tree rooted at dir and sends
// the size of each found file on filesizes
func walkDir(dir string, n *sync.WaitGroup, filesizes chan<- int64) {
	defer n.Done()
	for _, entry := range dirEntries(dir) {
		if entry.IsDir() {
			n.Add(1)
			subdir := filepath.Join(dir, entry.Name())
			go walkDir(subdir, n, filesizes)
		} else {
			filesizes <- entry.Size()
		}
	}
}

func dirEntries(dir string) []os.FileInfo {
	tokens <- struct{}{} // acquire token
	defer func() {
		<-tokens // release token
	}()

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return nil
	}

	return entries
}
