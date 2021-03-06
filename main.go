package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var usage = "go run <url> -rate=10"

type requestResult struct {
	res *http.Response
	err error
}

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	rate := flag.Int64("rate", 1, "Requests per second to send. Default: 1.")
	limit := flag.Int64("limit", 0, "Total requests to send. Default: send until interrupted.")
	method := flag.String("method", "get", "HTTP method")
	dataPath := flag.String("data", "", "Data to send (for now, must be a JSON file)")
	username := flag.String("username", "", "Basic auth username")
	password := flag.String("password", "", "Basic auth password")

	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(1)
	}
	uri := os.Args[len(os.Args)-1]

	upperMethod := strings.ToUpper(*method)

	// var bodyReader io.Reader
	var body []byte
	var err error
	if upperMethod == "POST" {
		if *dataPath != "" {
			body, err = ioutil.ReadFile(*dataPath)
			// bodyReader, err = os.Open(*dataPath)
			if err != nil {
				fmt.Printf("Failed to read file: %v", dataPath)
				os.Exit(1)
			}
		} else {
			fmt.Println("Must provide -data for POST requests")
			os.Exit(1)
		}
	}

	log.Printf("Testing %v %v with %v requests per second", upperMethod, uri, *rate)

	ticker := time.NewTicker(time.Second / (time.Duration(*rate)))
	results := make(chan requestResult)
	startTime := time.Now()

	requestCount := 0
	errorCount := 0
	var resultsByStatus = make(map[string]int)

	printResults := func() {
		elapsedSeconds := time.Since(startTime) / time.Second
		requestsPerSecond := requestCount / int(elapsedSeconds)
		log.Printf("Sent %v requests at %v requests/second. Counts by status code:", requestCount, requestsPerSecond)
		for status, count := range resultsByStatus {
			log.Printf("%v: %v", status, count)
		}
		log.Printf("Errors: %v", errorCount)
	}

	go func() {
		sig := <-sigs
		printResults()
		log.Printf("Exiting on signal %v", sig)
		os.Exit(0)
	}()

	client := &http.Client{}

	go func() {
		count := 0
		for _ = range ticker.C {
			req, err := http.NewRequest(upperMethod, uri, bytes.NewReader(body))
			if *username != "" || *password != "" {
				req.SetBasicAuth(*username, *password)
			}

			res, err := client.Do(req)

			results <- requestResult{
				res: res,
				err: err,
			}
			count++
			if *limit > 0 && int64(count) == *limit {
				printResults()
				os.Exit(0)
			}
		}
	}()

	for result := range results {
		requestCount++
		if count, ok := resultsByStatus[result.res.Status]; ok {
			resultsByStatus[result.res.Status] = count + 1
		} else {
			resultsByStatus[result.res.Status] = 1
		}

		if result.err != nil {
			errorCount++
			log.Printf("[ERR] %v %v: %v", upperMethod, uri, result.err)
		} else {
			log.Printf("[%v] %v %v", result.res.Status, upperMethod, uri)
		}
	}
}
