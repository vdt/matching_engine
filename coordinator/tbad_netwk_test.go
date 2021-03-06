package coordinator

import (
	. "github.com/fmstephe/matching_engine/msg"
	"github.com/fmstephe/matching_engine/q"
	"testing"
)

const TO_SEND = 1000

const (
	clientOriginId = iota
	serverOriginId = iota
)

type echoClient struct {
	AppMsgHelper
	received []*Message
	complete chan bool
}

func newEchoClient(complete chan bool) *echoClient {
	return &echoClient{received: make([]*Message, TO_SEND), complete: complete}
}

func (c *echoClient) Run() {
	go sendAll(c.Out)
	for {
		m := <-c.In
		if m.Kind == SHUTDOWN {
			return
		}
		if m != nil {
			if c.received[m.TradeId-1] != nil {
				panic("Duplicate message received")
			}
			c.received[m.TradeId-1] = m
			if full(c.received) {
				c.complete <- true
				return
			}
		}
	}
}

func full(received []*Message) bool {
	for _, rm := range received {
		if rm == nil {
			return false
		}
	}
	return true
}

func sendAll(out chan<- *Message) {
	for i := uint32(1); i <= TO_SEND; i++ {
		m := &Message{Kind: SELL, TraderId: 1, TradeId: i, StockId: 1, Price: 7, Amount: 1}
		out <- m
	}
}

type echoServer struct {
	AppMsgHelper
}

func (s *echoServer) Run() {
	for {
		m := <-s.In
		if m.Kind == SHUTDOWN {
			return
		}
		r := &Message{}
		*r = *m
		r.Kind = BUY
		s.Out <- r
	}
}

func testBadNetwork(t *testing.T, dropProb float64, cFunc CoordinatorFunc) {
	complete := make(chan bool)
	c := newEchoClient(complete)
	s := &echoServer{}
	clientToServer := q.NewMeddleQ("clientToServer", q.NewProbDropMeddler(dropProb))
	serverToClient := q.NewMeddleQ("serverToClient", q.NewProbDropMeddler(dropProb))
	cFunc(serverToClient, clientToServer, c, clientOriginId, "Client", false)
	cFunc(clientToServer, serverToClient, s, serverOriginId, "Server", false)
	<-complete
}
