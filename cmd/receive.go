package cmd

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type (
	messageCounter struct {
		counter     int64
	}

	receiveParams struct {
		showMessage bool
	}
)

func init() {
	receiveCmd.Flags().BoolVarP(&params.showMessage, "show-message", "s", false, "show each message")
	rootCmd.AddCommand(receiveCmd)
}

var (
	params     receiveParams
	receiveCmd = &cobra.Command{
		Use:   "receive",
		Short: "Receive messages from Service Bus",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAuthFlags()
		},
		Run: RunWithCtx(func(ctx context.Context, cmd *cobra.Command, args []string) {
			ns, err := servicebus.NewNamespace(servicebus.NamespaceWithConnectionString(connStr))
			if err != nil {
				log.Error(err)
				return
			}

			q, err := ns.NewQueue(entityPath)
			if err != nil {
				log.Error(err)
				return
			}
			defer func() {
				_ = q.Close(context.Background())
			}()

			ctx, span := tab.StartSpan(context.Background(), "receive.Run")
			defer span.End()

			if err := q.Receive(ctx, new(messageCounter)); err != nil {
				log.Error(err)
				return
			}

			<- ctx.Done()
			fmt.Println("received")
		}),
	}
)

func (mc *messageCounter) Handle(ctx context.Context, msg *servicebus.Message) error {
	ctx, span := tab.StartSpan(ctx, "messageCounter.Handle")
	defer span.End()

	atomic.AddInt64(&mc.counter, 1)
	if mc.counter % 10 == 0 {
		fmt.Printf("received %d messages \n", mc.counter)
		if params.showMessage {
			fmt.Println(string(msg.Data))
		}
	}
	return msg.Complete(ctx)
}