package matcher

import (
	"fmt"
	"github.com/fmstephe/matching_engine/coordinator"
	"github.com/fmstephe/matching_engine/matcher/pqueue"
	"github.com/fmstephe/matching_engine/msg"
)

type M struct {
	coordinator.AppMsgHelper
	matchQueues map[uint32]*pqueue.MatchQueues
	slab        *pqueue.Slab
}

func NewMatcher(slabSize int) *M {
	matchQueues := make(map[uint32]*pqueue.MatchQueues)
	slab := pqueue.NewSlab(slabSize)
	return &M{matchQueues: matchQueues, slab: slab}
}

func (m *M) Run() {
	for {
		o := <-m.In
		if o.Kind == msg.SHUTDOWN {
			m.Out <- o
			return
		}
		if o != nil {
			on := m.slab.Malloc()
			on.CopyFrom(o)
			switch on.Kind() {
			case msg.BUY:
				m.addBuy(on)
			case msg.SELL:
				m.addSell(on)
			case msg.CANCEL:
				m.cancel(on)
			default:
				panic(fmt.Sprintf("MsgKind %v not supported", on))
			}
		}
	}
}

func (m *M) getMatchQueues(stockId uint32) *pqueue.MatchQueues {
	q := m.matchQueues[stockId]
	if q == nil {
		q = &pqueue.MatchQueues{}
		m.matchQueues[stockId] = q
	}
	return q
}

func (m *M) addBuy(b *pqueue.OrderNode) {
	if b.Price() == msg.MARKET_PRICE {
		panic("It is illegal to send a buy at market price")
	}
	q := m.getMatchQueues(b.StockId())
	if !m.fillableBuy(b, q) {
		q.PushBuy(b)
	}
}

func (m *M) addSell(s *pqueue.OrderNode) {
	q := m.getMatchQueues(s.StockId())
	if !m.fillableSell(s, q) {
		q.PushSell(s)
	}
}

func (m *M) cancel(o *pqueue.OrderNode) {
	q := m.getMatchQueues(o.StockId())
	ro := q.Cancel(o)
	if ro != nil {
		completeCancelled(m.Out, ro)
		m.slab.Free(ro)
	} else {
		completeNotCancelled(m.Out, o)
	}
	m.slab.Free(o)
}

func (m *M) fillableBuy(b *pqueue.OrderNode, q *pqueue.MatchQueues) bool {
	for {
		s := q.PeekSell()
		if s == nil {
			return false
		}
		if b.Price() >= s.Price() {
			if b.Amount() > s.Amount() {
				amount := s.Amount()
				price := price(b.Price(), s.Price())
				m.slab.Free(q.PopSell())
				b.ReduceAmount(amount)
				completeTrade(m.Out, msg.PARTIAL, msg.FULL, b, s, price, amount)
				continue
			}
			if s.Amount() > b.Amount() {
				amount := b.Amount()
				price := price(b.Price(), s.Price())
				s.ReduceAmount(amount)
				completeTrade(m.Out, msg.FULL, msg.PARTIAL, b, s, price, amount)
				m.slab.Free(b)
				return true // The buy has been used up
			}
			if s.Amount() == b.Amount() {
				amount := b.Amount()
				price := price(b.Price(), s.Price())
				completeTrade(m.Out, msg.FULL, msg.FULL, b, s, price, amount)
				m.slab.Free(q.PopSell())
				m.slab.Free(b)
				return true // The buy has been used up
			}
		} else {
			return false
		}
	}
	panic("Unreachable")
}

func (m *M) fillableSell(s *pqueue.OrderNode, q *pqueue.MatchQueues) bool {
	for {
		b := q.PeekBuy()
		if b == nil {
			return false
		}
		if b.Price() >= s.Price() {
			if b.Amount() > s.Amount() {
				amount := s.Amount()
				price := price(b.Price(), s.Price())
				b.ReduceAmount(amount)
				completeTrade(m.Out, msg.PARTIAL, msg.FULL, b, s, price, amount)
				m.slab.Free(s)
				return true // The sell has been used up
			}
			if s.Amount() > b.Amount() {
				amount := b.Amount()
				price := price(b.Price(), s.Price())
				s.ReduceAmount(amount)
				completeTrade(m.Out, msg.FULL, msg.PARTIAL, b, s, price, amount)
				m.slab.Free(q.PopBuy())
				continue
			}
			if s.Amount() == b.Amount() {
				amount := b.Amount()
				price := price(b.Price(), s.Price())
				completeTrade(m.Out, msg.FULL, msg.FULL, b, s, price, amount)
				m.slab.Free(q.PopBuy())
				m.slab.Free(s)
				return true // The sell has been used up
			}
		} else {
			return false
		}
	}
	panic("Unreachable")
}

func price(bPrice, sPrice uint64) uint64 {
	if sPrice == msg.MARKET_PRICE {
		return bPrice
	}
	d := bPrice - sPrice
	return sPrice + (d / 2)
}

func completeTrade(out chan<- *msg.Message, brk, srk msg.MsgKind, b, s *pqueue.OrderNode, price uint64, amount uint32) {
	br := &msg.Message{Kind: brk, Price: price, Amount: amount, TraderId: b.TraderId(), TradeId: b.TradeId(), StockId: b.StockId()}
	sr := &msg.Message{Kind: srk, Price: price, Amount: amount, TraderId: s.TraderId(), TradeId: s.TradeId(), StockId: s.StockId()}
	out <- br
	out <- sr
}

func completeCancelled(out chan<- *msg.Message, c *pqueue.OrderNode) {
	cm := &msg.Message{}
	c.CopyTo(cm)
	cm.Kind = msg.CANCELLED
	out <- cm
}

func completeNotCancelled(out chan<- *msg.Message, nc *pqueue.OrderNode) {
	ncm := &msg.Message{}
	nc.CopyTo(ncm)
	ncm.Kind = msg.NOT_CANCELLED
	out <- ncm
}
