package cmd

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loadSingleCmd)

	loadSingleParams.numberOfClients = 5000
}

type (
	LoadSingleParams struct {
		numberOfClients int
		clients         []*servicebus.Queue
	}
)

var (
	loadSingleParams LoadSingleParams
	loadSingleCmd    = &cobra.Command{
		Use:   "load-single",
		Short: "Create multiple clients in loop with single sender and receiver connecting to the same queue.",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			entityPath = generateQueueName()
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "loadSingle.Run")
			defer span.End()
			ctx, runCancel := context.WithCancel(ctx)
			defer runCancel()

			ns, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
			if err != nil {
				log.Error(err)
				return
			}

			_, err = ensureQueue(ctx, ns, entityPath)
			if err != nil {
				log.Error(err)
				return
			}

			go loadSingleSnapshot()

			func() {
				var num = 0
				for num < loadSingleParams.numberOfClients {
					q, err := ns.NewQueue(entityPath)
					if err != nil {
						log.Error(err)
						return
					}
					loadSingleParams.clients = append(loadSingleParams.clients, q)
					err = q.Send(ctx, servicebus.NewMessageFromString("test"))
					if err != nil {
						log.Error(err)
					}

					err = q.ReceiveOne(ctx, servicebus.HandlerFunc(func(ctx context.Context, msg *servicebus.Message) error {
						ctx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
						defer cancel()

						log.Printf("Received: %q", msg.ID)
						return msg.Complete(ctx)
					}))
					if err != nil {
						log.Error(err)
					}
					num++
				}
			}()

			defer func() {
				for i := 0; i < len(loadSingleParams.clients); i++ {
					err = loadSingleParams.clients[i].Close(ctx)
					if err != nil {
						log.Error(err)
					}
					cleanupQueue(ctx, ns, loadMultipleParams.clients[i].Name)
				}
			}()

			// Wait for a signal to quit:
			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, os.Kill)

			select {
			case <-signalChan:
				log.Println("closing via OS signal...")
				runCancel()
				return
			}
		},
	}
)

func loadSingleSnapshot() {
	for range time.Tick(time.Minute) {
		log.Infof("Number of clients created so far : %d", len(loadSingleParams.clients))
	}
}
