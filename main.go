package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Patterns to search for in the response body
var patterns = []string{
	// Linux patterns
	"root:",             // /etc/passwd
	"root:x",            // /etc/group
	"localhost",         // /etc/hosts or Windows hosts file
	"HTTP_USER_AGENT",   // /proc/self/environ
	"Linux version",     // /proc/version
	"Accepted password", // /var/log/auth.log
	"Failed password",   // /var/log/auth.log
	"GET /",             // Web server logs
	"POST /",            // Web server logs

	// Windows patterns
	"Administrator",        // SAM file
	"Guest",                // SAM file
	"HKEY_LOCAL_MACHINE",   // Registry file
	"default=",             // boot.ini
	"disable_functions",    // php.ini
	"safe_mode",            // php.ini
	"datadir=",             // my.cnf or my.ini
	"DocumentRoot",         // httpd.conf
	"Listen",               // httpd.conf
	"server_name",          // nginx.conf
	"worker_processes",     // nginx.conf
	"PermitRootLogin",      // sshd_config
	"Port",                 // sshd_config
	"<?php",                // PHP source code
	"<%",                  // JSP/ASP source code
	"$_GET",                // PHP superglobals
	"$_POST",               // PHP superglobals
	"$_SERVER",             // PHP superglobals
	"request.getParameter", // JSP source code
}

var (
	verbose    bool
	timeout    time.Duration
	numWorkers int
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Display detailed error messages")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "Request timeout duration")
	flag.IntVar(&numWorkers, "workers", 10, "Number of concurrent workers")
	flag.Parse()
}

func main() {
	var urls []string
	if len(flag.Args()) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" {
				urls = append(urls, url)
			}
		}
		if err := scanner.Err(); err != nil && verbose {
			fmt.Printf("Error reading stdin: %s\n", err)
		}
	} else {
		urls = append(urls, flag.Arg(0))
	}

	urlChan := make(chan string, len(urls))
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(urlChan, &wg)
	}

	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	wg.Wait()
}

func worker(urlChan <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	client := &http.Client{
		Timeout: timeout,
	}

	for url := range urlChan {
		processURL(client, url)
	}
}

func processURL(client *http.Client, url string) {
	resp, err := client.Get(url)
	if err != nil {
		if verbose {
			fmt.Printf("Error fetching URL %s: %s\n", url, err)
		}
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	foundPatterns := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()

		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				foundPatterns[pattern] = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if verbose {
			fmt.Printf("Error reading response from URL %s: %s\n", url, err)
		}
		return
	}

	if len(foundPatterns) > 0 {
		fmt.Printf("Detected patterns in %s:\n", url)
		for pattern := range foundPatterns {
			fmt.Printf("- %s\n", pattern)
		}
	} else if verbose {
		fmt.Printf("No patterns detected in %s.\n", url)
	}
}
