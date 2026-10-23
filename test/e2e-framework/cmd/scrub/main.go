// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Command scrub runs a command, scrubbing credentials from its output with the default scrubber:
//
//	scrub <command> [<args>...]
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/DataDog/datadog-agent/pkg/util/scrubber"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: scrub <command> [<args>...]")
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fail(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fail(err)
	}
	if err := cmd.Start(); err != nil {
		fail(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range signals {
			_ = cmd.Process.Signal(sig) // unsupported on Windows, whose console delivers interrupts to the whole group
		}
	}()

	var copies sync.WaitGroup
	copies.Add(2)
	go scrub(stdout, os.Stdout, &copies)
	go scrub(stderr, os.Stderr, &copies)
	copies.Wait() // before cmd.Wait, which closes the pipes

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitErr.ExitCode())
		}
		fail(err)
	}
}

func scrub(in io.Reader, out io.Writer, done *sync.WaitGroup) {
	defer done.Done()
	reader := bufio.NewReader(in) // unlike bufio.Scanner, not limited in line length
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if _, err := io.WriteString(out, scrubber.ScrubLine(line)); err != nil {
				fail(err)
			}
		}
		if err != nil {
			return
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "scrub:", err)
	os.Exit(1)
}
