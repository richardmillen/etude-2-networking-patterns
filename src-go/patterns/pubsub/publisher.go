package pubsub

import (
	"log"
	"sync"

	"github.com/richardmillen/etude-2-net-patterns/src-go/patterns/core"
)

// NewPublisher returns a new Publisher that will publish messages to Subscriber's.
func NewPublisher(c core.Connector) *Publisher {
	pub := &Publisher{connector: c}

	pub.connector.OnConnect(pub.onNewConn)
	pub.ch = make(chan Message, pub.connector.QueueSize())
	pub.err = make(chan error, pub.connector.QueueSize())
	pub.quit = make(chan bool, 1)
	pub.finished = make(chan bool)

	go pub.run()

	return pub
}

// Publisher sends messages to zero or more Subscriber's.
type Publisher struct {
	connector core.Connector
	ch        chan Message
	quit      chan bool
	err       chan error
	finished  chan bool
	wgSend    sync.WaitGroup
}

// run is the engine of the Publisher.
//
// note that the select case/default as opposed to select case/case
// where the latter includes the quit channel. this is to ensure the
// pub channel is flushed before responding to the quit channel. put
// another way, several messages could be queued in pub.ch then the
// application could close, causing an event on pub.quit. this would
// mean that anything in the pub.ch queue would be lost.
// n.b. if this behaviour is desirable then it should still be possible
// by configuring the connector to quit before the Publisher. GetQueues
// could be made to return nil for instance.
//
// refer to the language spec for furter info on select case/case vs
// select case/default:
// https://golang.org/ref/spec#Select_statements
//
// TODO: should we report queue errors to the consumer?
func (pub *Publisher) run() {
	defer func() {
		log.Println("publisher finished.")
		pub.finished <- true
	}()

	err := pub.connector.Open(&pubProtoV1{})
	pub.setError(err)

	for {
		select {
		case m := <-pub.ch:
			core.SendToQueues(pub.connector, &m)
		case <-pub.quit:
			core.CloseQueues(pub.connector)
			return
		}
	}
}

func (pub *Publisher) setError(err error) bool {
	if err == nil {
		return false
	}

	select {
	case pub.err <- err:
		return true
	default:
		return false
	}
}

// onNewConn is invoked by the Publishers Connector whenever a new connection Queue is created.
func (pub *Publisher) onNewConn(q *core.Queue) error {
	// TODO: forward this on to the consumer of the API.
	return nil
}

// Publish sends data to subscribers.
func (pub *Publisher) Publish(topic string, content []byte) error {
	select {
	case err := <-pub.err:
		return err
	default:
	}

	core.SendToQueues(pub.connector, &Message{Topic: topic, Body: content})
	return nil

	// TODO: reinstate the channel-based logic.
	/*pub.wgSend.Add(1)

	select {
	case pub.ch <- Message{Topic: topic, Body: content}:
		fmt.Println("Publisher.Publish: message sent to channel.")
		return nil
	default:
		pub.wgSend.Done()
		return errors.New("publisher queue full")
	}*/
}

// Close is called to stop and invalidate the Publisher.
func (pub *Publisher) Close() error {
	pub.wgSend.Wait()

	pub.quit <- true
	<-pub.finished
	return nil
}
