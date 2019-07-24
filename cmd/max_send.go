package cmd

import (
	"context"
	"github.com/Azure/azure-amqp-common-go/v2/uuid"
	servicebus "github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
	"os/signal"
)

func init() {
	maxSendCmd.Flags().IntVar(&msParams.messageSize, "message-size", 1024, "size of messages")
	maxSendCmd.Flags().IntVar(&msParams.numberOfSenders, "num-senders", 10, "number of senders")

	rootCmd.AddCommand(maxSendCmd)
}

type (
	repeatSender struct {
		namespace   string
		queueName   string
		messageSize int
	}

	maxSendParams struct {
		messageSize     int
		numberOfSenders int
	}
)

func newRepeatSender(messageSize int, namespace, queueName string) *repeatSender {
	return &repeatSender{
		namespace:   namespace,
		queueName:   queueName,
		messageSize: messageSize,
	}
}

func (s *repeatSender) Run(ctx context.Context, ns *servicebus.Namespace, sentChan chan string, errChan chan error) {
	queue, err := ns.NewQueue(s.queueName)
	if err != nil {
		errChan <- err
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			id, err := uuid.NewV4()
			if err != nil {
				errChan <- err
				return
			}

			batchSize := 256000 / msParams.messageSize
			messages := make([]*servicebus.Message, batchSize)
			for i := 0; i < batchSize; i++ {
				data := make([]byte, msParams.messageSize)
				_, _ = rand.Read(data)
				message := servicebus.NewMessage(data)
				message.ID = id.String()
				messages[i] = message
			}

			err = queue.SendBatch(ctx, servicebus.NewMessageBatchIterator(servicebus.StandardMaxMessageSizeInBytes, messages...))

			if err != nil {
				errChan <- err
				return
			}

			for i := 0; i < batchSize; i++ {
				sentChan <- messages[i].ID
			}
		}
	}
}

var (
	msParams maxSendParams

	maxSendCmd = &cobra.Command{
		Use:   "max-send",
		Short: "Send messages in parallel of a given size",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			entityPath = generateQueueName("max-send")
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "maxSend.Run")
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

			errChan := make(chan error, 1)
			defer close(errChan)
			sentChan := make(chan string, 10)
			defer close(sentChan)

			for i := 0; i < msParams.numberOfSenders; i++ {
				sender := newRepeatSender(msParams.messageSize, namespace, entityPath)
				go sender.Run(ctx, ns, sentChan, errChan)
			}

			// Wait for a signal to quit:
			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, os.Kill)

			count := 0
			for {
				select {
				case <-signalChan:
					log.Println("closing via OS signal...")
					runCancel()
					return
				case err := <-errChan:
					log.Error(err)
					runCancel()
					return
				case _ = <-sentChan:
					count++
					if count%10000 == 0 {
						log.Printf("Sent: %d", count)
					}
				}
			}
		},
	}
)
