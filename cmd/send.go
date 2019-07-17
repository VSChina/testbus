package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	sendCmd.Flags().IntVar(&sendParams.messageCount, "msg-count", 10, "number of messages to send")
	sendCmd.Flags().IntVar(&sendParams.messageSize, "msg-size", 256, "size in bytes of each message")
	rootCmd.AddCommand(sendCmd)
}

type (
	SendParams struct {
		messageSize  int
		messageCount int
	}
)

var (
	sendParams SendParams
	sendCmd    = &cobra.Command{
		Use:   "send",
		Short: "Send messages to an Event Hub",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			return checkAuthFlags()
		},
		Run: RunWithCtx(func(ctx context.Context, cmd *cobra.Command, args []string) {
			ns, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
			if err != nil {
				tab.For(ctx).Error(err)
				return
			}

			q, err := ns.NewQueue(entityPath)
			if err != nil {
				tab.For(ctx).Error(err)
				return
			}

			log.Println(fmt.Sprintf("attempting to send %d messages", sendParams.messageCount))
			sentMsgs := 0

			sendMsg := func(data []byte) error {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				ctx, span := tab.StartSpan(ctx, "sendMsg")
				defer span.End()

				err := q.Send(ctx, servicebus.NewMessage(data))
				if err != nil {
					tab.For(ctx).Error(err)
					return err
				}
				sentMsgs++
				return nil
			}

			for i := 0; i < sendParams.messageCount; i++ {
				data := make([]byte, sendParams.messageSize)
				_, err := rand.Read(data)
				if err != nil {
					tab.For(ctx).Error(err)
					continue
				}
				if err := sendMsg(data); err != nil {
					tab.For(ctx).Error(err)
					return
				}
			}

			log.Printf("sent %d messages\n", sentMsgs)
		}),
	}
)
