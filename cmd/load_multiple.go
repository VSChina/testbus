package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loadMultipleCmd)

	loadMultipleParams.numberOfClients = 5000
	loadMultipleParams.waitGroup.Add(1)
}

type (
	LoadMultipleParams struct {
		numberOfClients int
		clients         []*servicebus.Queue
		waitGroup       sync.WaitGroup
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
			entityPath = generateQueueName("load-multiple")
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "loadMultiple.Run")
			defer span.End()
			ctx, runCancel := context.WithCancel(ctx)
			defer runCancel()

			ns, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
			if err != nil {
				log.Error(err)
				return
			}

			go loadMultipleSnapshot(ctx)

			go createMultipleClients(ctx, ns)

			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, os.Kill)

			go func() {
				<-signalChan
				log.Println("closing via OS signal...")
				cleanupMultipleClients(ctx, ns)
				runCancel()
				return
			}()

			defer cleanupMultipleClients(ctx, ns)
			loadMultipleParams.waitGroup.Wait()
		},
	}
)

func cleanupMultipleClients(ctx context.Context, ns *servicebus.Namespace) {
	for i := 0; i < len(loadMultipleParams.clients); i++ {
		err := loadMultipleParams.clients[i].Close(ctx)
		if err != nil {
			log.Error(err)
		}
		cleanupQueue(ctx, ns, loadMultipleParams.clients[i])
	}
}

func createMultipleClients(ctx context.Context, ns *servicebus.Namespace) {
	defer loadMultipleParams.waitGroup.Done()
	var num = 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if num < loadMultipleParams.numberOfClients {
				queueName := fmt.Sprintf(entityPath+"-%d", num)
				_, err := ensureQueue(ctx, ns, queueName)
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
			} else {
				return
			}
		}
	}
}

func loadMultipleSnapshot(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(time.Minute)
			log.Infof("Number of clients created so far : %d", len(loadMultipleParams.clients))
		}
	}
}
