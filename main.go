package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"

	hookClient "github.com/labbsr0x/sandman-dns-webhook/src/client"
	"github.com/labbsr0x/sandman-dns-webhook/src/types"
	"github.com/labbsr0x/sandman-swarm-listener/docker"
)

const (
	// ErrInitDockerClient error code for problems while creating the Docker Client
	ErrInitDockerClient = iota

	// ErrInitHookClient error code for problems while creating the Hook Client
	ErrInitHookClient = iota

	// ErrTalkToDocker
	ErrTalkToDocker = iota
)

func main() {
	dockerClient, err := docker.NewEnvClient()
	types.PanicIfError(types.Error{Message: fmt.Sprintf("Not possible to start the swarm listener; something went wrong while creating the Docker Client: %s", err), Code: 1, Err: err})

	hookClient, err := hookClient.New()
	types.PanicIfError(types.Error{Message: fmt.Sprintf("Not possible to start the swarm listener; something went wrong while creating the sandman dns manager hook client: %s", err), Code: 2, Err: err})

	listen(dockerClient, hookClient) // fire and forget
}

// listen prepares the ground to listen to docker events. it blocks the main thread keeping it alive
func listen(dockerClient *docker.Client, hookClient *hookClient.DNSWebhookClient) {
	listeningCtx, cancel := context.WithCancel(context.Background())
	events, errs := dockerClient.Events(listeningCtx, docker.EventsOptions{})
	go handleMessages(listeningCtx, events)
	go handleErrors(listeningCtx, errs, cancel)
	go gracefulStop(cancel)
	select {} // keep alive magic
}

// handleMessages deals with the event messages being dispatched by the docker swarm cluster
func handleMessages(ctx context.Context, events <-chan docker.Message) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("Stopping message handler")
			return
		case message := <-events:
			fmt.Println("Message received: ", message)

		}
	}
}

// gracefullStop cancels gracefully the running goRoutines
func gracefulStop(cancel context.CancelFunc) {
	stopCh := make(chan os.Signal)

	signal.Notify(stopCh, syscall.SIGTERM)
	signal.Notify(stopCh, syscall.SIGINT)

	<-stopCh // waits for a stop signal
	stop(0, cancel)
}

// handleErrors deals with errors dispatched in the communication with the docker swarm cluster
func handleErrors(ctx context.Context, errs <-chan error, cancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("Stopping error handler")
			return
		case err := <-errs:
			logrus.Errorf("Error communicating with the docker swarm cluster: %s", err)
			stop(3, cancel)
		}
	}
}

// stops the whole listener
func stop(returnCode int, cancel context.CancelFunc) {
	logrus.Infof("Stopping routines...")
	cancel()
	os.Exit(returnCode)
}
