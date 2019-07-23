package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loadMultipleCmd)

	loadMultipleParams.numberOfClients = 5000
}

type (
	LoadMultipleParams struct {
		numberOfClients int
		clients         []*servicebus.Queue
	}
)

var (
	loadMultipleParams LoadMultipleParams
	loadMultipleCmd    = &cobra.Command{
		Use:   "load-multiple",
		Short: "Create multiple clients in loop with single sender and receiver connecting to different queues.",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			entityPath = generateQueueName()
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "loadMultiple.Run")
			defer span.End()

			ns, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
			if err != nil {
				log.Error(err)
				return
			}

			go loadMultipleSnapshot()

			func() {
				var num = 0
				for num < loadMultipleParams.numberOfClients {
					queueName := fmt.Sprintf(entityPath+"-%d", num)
					_, err = ensureQueue(ctx, ns, queueName)
					if err != nil {
						log.Error(err)
						return
					}

					q, err := ns.NewQueue(queueName)
					if err != nil {
						log.Error(err)
						return
					}
					loadMultipleParams.clients = append(loadMultipleParams.clients, q)
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
				for i := 0; i < len(loadMultipleParams.clients); i++ {
					err = loadMultipleParams.clients[i].Close(ctx)
					if err != nil {
						log.Error(err)
					}
					cleanupQueue(ctx, ns, loadMultipleParams.clients[i].Name)
				}
			}()
		},
	}
)

func loadMultipleSnapshot() {
	for range time.Tick(time.Minute) {
		log.Infof("Number of clients created so far : %d", len(loadMultipleParams.clients))
	}
}
