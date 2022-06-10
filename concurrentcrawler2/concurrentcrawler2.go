// This version uses a buffered channel as a counting semaphore
// to limit the number of concurrent calls to link.Extract.
package main

import (
	"fmt"
	"gopl/links"
	"log"
	"os"
)

// counting semaphore to enfore a limit of 20 concurrent requests
var tokens = make(chan struct{}, 20)

func crawl(url string) []string {
	fmt.Println(url)

	tokens <- struct{}{} // acquire a token
	list, err := links.Extract(url)
	<-tokens // release a token

	if err != nil {
		log.Print(err)
	}

	return list
}

func main() {
	worklist := make(chan []string)
	var n int // number of pending sends to worklist

	// start with the command-line arguments
	n++
	go func() {
		worklist <- os.Args[1:]
	}()

	// crawl the web concurrently
	seen := make(map[string]bool)
	for ; n > 0; n-- {
		list := <-worklist
		for _, link := range list {
			if !seen[link] {
				seen[link] = true
				n++
				go func(link string) {
					worklist <- crawl(link)
				}(link)
			}
		}
	}
}
