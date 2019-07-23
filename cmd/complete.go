package cmd

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type (
	completeContext struct {
		queue      *servicebus.Queue
		sentMsgs   int
		isFinished bool
		waitGroup  sync.WaitGroup
		messageSet map[string]bool
	}

	completeHandler struct{}
)

func init() {
	rootCmd.AddCommand(completeCmd)

	completeCxt.sentMsgs = 0
	completeCxt.isFinished = false
	completeCxt.waitGroup.Add(2)
	completeCxt.messageSet = make(map[string]bool)
}

var (
	completeCxt completeContext
	completeCmd = &cobra.Command{
		Use:   "complete",
		Short: "Send a message, receive message and complete it",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			entityPath = generateQueueName()
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "complete.Run")
			defer span.End()

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

			q, err := ns.NewQueue(entityPath)
			if err != nil {
				log.Error(err)
				return
			}
			completeCxt.queue = q

			go receiveMsg()
			time.Sleep(3 * time.Second)
			go sendMsg(ctx)

			go completeSnapshot()

			completeCxt.waitGroup.Wait()

			_ = completeCxt.queue.Close(ctx)
			cleanupQueue(ctx, ns, entityPath)
		},
	}
)

func sendMsg(ctx context.Context) {
	ctx, span := tab.StartSpan(ctx, "complete.sendMsg")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, (testDurationInMs+5000)*time.Second)
	defer cancel()
	messageID := 1

	for !completeCxt.isFinished {
		message := servicebus.NewMessageFromString("test")
		message.ID = strconv.Itoa(messageID)
		err := completeCxt.queue.Send(ctx, message)
		if err != nil {
			log.Error(err)
			return
		}
		completeCxt.messageSet[message.ID] = true
		messageID++
		completeCxt.sentMsgs++
		time.Sleep(500 * time.Millisecond) // Throttling send to not increase queue size
	}
	log.Printf("sent %d messages\n", completeCxt.sentMsgs)
	completeCxt.waitGroup.Done()
}

func receiveMsg() {
	ctx, span := tab.StartSpan(context.Background(), "complete.receive.Run")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, testDurationInMs*time.Millisecond)
	defer cancel()

	err := completeCxt.queue.Receive(ctx, new(completeHandler))
	if err != nil {
		log.Error(err)
	}

	completeCxt.isFinished = true
	completeCxt.waitGroup.Done()
}

func (mc *completeHandler) Handle(ctx context.Context, msg *servicebus.Message) error {
	ctx, span := tab.StartSpan(ctx, "complete.messageCounter.Handle")
	defer span.End()

	if !completeCxt.messageSet[msg.ID] {
		return errors.New("received message that is not recorded in internal map")
	}
	delete(completeCxt.messageSet, msg.ID)
	id, _ := strconv.Atoi(msg.ID)
	if id%1000 == 0 {
		log.Infof("received %d messages", id)
	}
	return msg.Complete(ctx)
}

func completeSnapshot() {
	for range time.Tick(time.Minute) {
		log.Infof("Map size: %d, Number of messages sent and received successfully so far : %d",
			len(completeCxt.messageSet), completeCxt.sentMsgs)
	}
}
