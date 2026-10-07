package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

var version = "dev"

func main() {
	interval := flag.Duration("d", 2*time.Second, "refresh interval")
	showVersion := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("itop", version)
		return
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "itop only runs on linux")
		os.Exit(1)
	}
	if *interval < 500*time.Millisecond {
		*interval = 500 * time.Millisecond
	}

	a := newApp(*interval)
	go a.loop()
	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
