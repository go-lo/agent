package main

import (
	"time"
)

func main() {
	c := Client{
		Duration: 5 * time.Second,
	}

	c.Start()
}
