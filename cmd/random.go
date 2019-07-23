package cmd

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/Azure/azure-service-bus-go"
	"github.com/devigned/tab"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type (
	randomContext struct {
		queue            *servicebus.Queue
		sentMsgs         int
		isFinished       bool
		waitGroup        sync.WaitGroup
		abandonCount     int
		completeCount    int
		deadletterCount  int
		deferCount       int
		messageToProcess map[string]bool
		messageAbandoned map[string]int
	}

	randomHandler struct{}
)

func init() {
	rootCmd.AddCommand(randomCmd)

	randomCxt.sentMsgs = 0
	randomCxt.isFinished = false
	randomCxt.waitGroup.Add(2)
	randomCxt.abandonCount = 0
	randomCxt.completeCount = 0
	randomCxt.deadletterCount = 0
	randomCxt.deferCount = 0
	randomCxt.messageToProcess = make(map[string]bool)
	randomCxt.messageAbandoned = make(map[string]int)
}

var (
	randomCxt randomContext
	randomCmd = &cobra.Command{
		Use:   "random",
		Short: "Send a message, receive message and random disposition on a queue",
		Args: func(cmd *cobra.Command, args []string) error {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			entityPath = generateQueueName()
			return checkAuthFlags()
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, span := tab.StartSpan(context.Background(), "random.Run")
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
			randomCxt.queue = q

			go receiveRandomMsg()
			time.Sleep(3 * time.Second)
			go sendRandomMsg(ctx)
			go randomSnapshot()
			randomCxt.waitGroup.Wait()

			_ = randomCxt.queue.Close(ctx)
			cleanupQueue(ctx, ns, entityPath)
		},
	}
)

func sendRandomMsg(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, (testDurationInMs+5000)*time.Millisecond)
	defer cancel()
	ctx, span := tab.StartSpan(ctx, "random.sendMsg")
	defer span.End()
	messageID := 1

	for !randomCxt.isFinished {
		message := servicebus.NewMessageFromString("test")
		message.ID = string(messageID)
		err := randomCxt.queue.Send(ctx, message)
		if err != nil {
			log.Error(err)
			return
		}
		randomCxt.messageToProcess[message.ID] = true
		randomCxt.messageAbandoned[message.ID] = 0
		messageID++
		randomCxt.sentMsgs++
		time.Sleep(2 * time.Second) // Throttling send to not increase queue size
	}
	log.Printf("sent %d messages\n", randomCxt.sentMsgs)
	randomCxt.waitGroup.Done()
}

func receiveRandomMsg() {
	ctx, span := tab.StartSpan(context.Background(), "random.receive.Run")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, testDurationInMs*time.Millisecond)
	defer cancel()

	err := randomCxt.queue.Receive(ctx, new(randomHandler))
	if err != nil {
		log.Error(err)
	}

	randomCxt.isFinished = true
	randomCxt.waitGroup.Done()
}

func (mc *randomHandler) Handle(ctx context.Context, msg *servicebus.Message) error {
	ctx, span := tab.StartSpan(ctx, "random.messageCounter.Handle")
	defer span.End()

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	seed := r1.Intn(4)
	switch seed {
	case 0:
		currCount := randomCxt.messageAbandoned[msg.ID]
		if currCount == 10 {
			randomCxt.abandonCount++
			if randomCxt.messageToProcess[msg.ID] {
				delete(randomCxt.messageToProcess, msg.ID)
			}
		}
		randomCxt.messageAbandoned[msg.ID] = currCount + 1
		return msg.Abandon(ctx)
	case 1:
		randomCxt.completeCount++
		if randomCxt.messageToProcess[msg.ID] {
			delete(randomCxt.messageToProcess, msg.ID)
		}
		return msg.Complete(ctx)
	case 2:
		randomCxt.deadletterCount++
		if randomCxt.messageToProcess[msg.ID] {
			delete(randomCxt.messageToProcess, msg.ID)
		}
		return msg.DeadLetter(ctx, errors.New("dead letter error"))
	case 3:
		randomCxt.deferCount++
		if randomCxt.messageToProcess[msg.ID] {
			delete(randomCxt.messageToProcess, msg.ID)
		}
		return msg.Defer(ctx)
	default:
		return errors.New("unexpected seed")
	}
}

func randomSnapshot() {
	for range time.Tick(time.Minute) {
		log.Infof(`Number of messages not processed yet : %d
Number of messages sent so far : %d
Number of messages abandoned : %d
Number of messages completed : %d
Number of messages deadlettered : %d
Number of messages deferred : %d`,
			len(randomCxt.messageToProcess), randomCxt.sentMsgs, randomCxt.abandonCount,
			randomCxt.completeCount, randomCxt.deadletterCount, randomCxt.deferCount)
	}
}
