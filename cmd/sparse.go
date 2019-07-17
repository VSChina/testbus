package cmd

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/Azure/azure-amqp-common-go/v2/uuid"
	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(sparseCmd)
}

type (
	uniformDistributedPeriodicSender struct {
		q          *servicebus.Queue
		maxSeconds int
	}
)

func newUniformDistributedPeriodicSender(maxSeconds int, q *servicebus.Queue) *uniformDistributedPeriodicSender {
	return &uniformDistributedPeriodicSender{
		maxSeconds: maxSeconds,
		q:          q,
	}
}

func (u *uniformDistributedPeriodicSender) Run(ctx context.Context, errChan chan error) {
	defer close(errChan)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// send a pair of messages on the same link spread across a variable amount of time
			err := u.sendPair(ctx)
			if err != nil {
				errChan <- err
				return
			}
		}
	}
}

func (u *uniformDistributedPeriodicSender) sendPair(ctx context.Context) error {
	ctx, spanner := tab.StartSpan(ctx, "uniformDistributedPeriodicSender.sendPair")
	defer spanner.End()

	err := u.send(ctx)
	if err != nil {
		tab.For(ctx).Error(err)
		return err
	}

	delay := time.Duration(rand.Intn(u.maxSeconds)) * time.Second
	log.Printf("next send at: %q", time.Now().Add(delay).Format("2006-01-02 15:04:05"))
	time.Sleep(delay)

	err = u.send(ctx)
	if err != nil {
		tab.For(ctx).Error(err)
		return err
	}

	return nil
}

func (u *uniformDistributedPeriodicSender) send(ctx context.Context) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}
	msg := servicebus.NewMessageFromString(id.String())
	msg.ID = id.String()
	err = u.q.Send(ctx, msg)
	if err != nil {
		return err
	}
	log.Printf("Sent: %q", msg.ID)
	return nil
}

var (
	//params sparseParams

	sparseCmd = &cobra.Command{
		Use:   "sparse-test",
		Short: "Send and Receive sparse messages over a long period of time",
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
				log.Error(err)
				return
			}

			if _, err := ensureQueue(ctx, ns, entityPath); err != nil {
				log.Error(err)
				tab.For(ctx).Error(err)
				return
			}

			q, err := ns.NewQueue(entityPath)
			if err != nil {
				tab.For(ctx).Error(err)
				log.Error(err)
				return
			}

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			exploded := make(chan error, 1)
			go func() {
				err := q.Receive(ctx, servicebus.HandlerFunc(func(ctx context.Context, msg *servicebus.Message) error {
					log.Printf("Received: %q", msg.ID)
					return msg.Complete(ctx)
				}))

				if err != nil {
					exploded <- err
				}

				exploded <- errors.New("receive ended, and there was no error")
			}()

			sender := newUniformDistributedPeriodicSender(1800, q)
			go sender.Run(ctx, exploded)

			select {
			case <-ctx.Done():
				log.Println("closing with context done")
				return
			case err := <-exploded:
				log.Error(err)
				tab.For(ctx).Error(err)
				if ctx.Err() != nil {
					tab.For(ctx).Error(ctx.Err())
				}
				cancel()
				return
			}
		}),
	}
)
