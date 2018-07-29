package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/go-lo/go-lo"
)

const (
	// DeadLetterDatabase is the place to send data when a job
	// name hasn't been specified
	DeadLetterDatabase = "dead_letter"
)

var (
	collector = flag.String("collector", "http://localhost:8082", "Collector endpoint")
	insecure  = flag.Bool("insecure", false, "Allow access to https endpoints with invalid certs/ certificate chains")
	logDir    = flag.String("logs", "/var/log/loadtest-agent", "Dir to log to")

	cooldownSeconds = 60.0
	silent          bool
)

func main() {
	flag.Parse()

	api, collector, err := Setup(*insecure, *collector, "/tmp")
	if err != nil {
		panic(err)
	}

	go JobHandler(collector, api.Jobs)

	panic(http.ListenAndServe(":8081", api))
}

// Setup prepares an agent to be run by
// validating and setting it's internal http client,
// API Server, and initialising a job queue
func Setup(insecure bool, collectorEndpoint, binariesDir string) (a API, c Collector, err error) {
	if insecure {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	j := make(chan Job, 32)

	a = API{
		UploadDir: binariesDir,
		Jobs:      j,
		Binaries:  NewBinaries(),
	}

	log.Print(collectorEndpoint)
	c, err = NewCollector(collectorEndpoint, DeadLetterDatabase)

	return
}

// JobHandler iterates over a job queue and runs them
// It first executes a job, then listens to its output chan
// which it passes on to the collector client to deal with.
//
// It also ensures that all outputs are dealt with before
// closing the output chan by enforcing a cooldown period. This
// cooldown period is an amount of time to receive no data before
// accepting the job is over and dealt with
func JobHandler(collector Collector, jobs chan Job) {
	for j := range jobs {
		if j.Name == "" {
			j.Name = DeadLetterDatabase
		}
		collector.Database = j.Name

		outputs := make(chan golo.Output)

		var lastRead time.Time
		go func() {
			for o := range outputs {
				lastRead = time.Now()

				err := collector.Push(o)
				if err != nil && !silent {
					log.Print(err)
				}
			}
		}()

		err := j.Start(outputs)
		if err != nil {
			log.Print(err)
		}

		log.Print("Entering cooldown period")

		// Wait until we've received nothing for a minute
		// in the hopes that this is enough for the final
		// requests to end
		for {
			if time.Now().Sub(lastRead).Seconds() > cooldownSeconds {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		close(outputs)
		log.Printf("ran %d times", j.items)
	}
}
