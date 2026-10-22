/*
	the host role, qahyacli run <job.yaml>. it wires together the pieces:
		loading the job
		creating the store
		starting the socket server
		running the steps
*/
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"qahyacli/internal/jobapi"
	"qahyacli/internal/runner"
	"qahyacli/internal/state"
)

func cmdRun(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: qahyacli run <job.yaml>")
	}

	job, err := runner.LoadJob(args[0])
	if err != nil {
		return err
	}

	// Ctrl-C cancels the running step and lets the deferred Close below run,
	// so the socket dir is removed instead of left behind
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := state.NewStore()
	srv, err := jobapi.Start(store)
	if err != nil {
		return err
	}
	// stop serving only after the last step has finished
	defer srv.Close()

	return runner.Run(ctx, job, store, srv.SocketPath())
}
