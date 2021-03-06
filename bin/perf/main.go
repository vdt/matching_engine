package main

import (
	"flag"
	"github.com/fmstephe/fstrconv"
	"github.com/fmstephe/matching_engine/matcher"
	"github.com/fmstephe/matching_engine/msg"
	"log"
	"math/rand"
	"os"
	"runtime/pprof"
	"time"
)

const (
	StockId = uint32(1)
)

var (
	filePath   = flag.String("f", "", "Relative path to an ITCH file providing test data")
	profile    = flag.String("p", "", "Write out a profile of this application, 'cpu' and 'mem' supported")
	orderNum   = flag.Int("o", 100, "The number of orders to generate. Ignored if -f is provided")
	delDelay   = flag.Int("d", 10, "The number of orders generated before we begin deleting existing orders")
	perfRand   = rand.New(rand.NewSource(1))
	orderMaker = msg.NewMessageMaker(1)
)

func main() {
	doPerf(true)
}

func doPerf(log bool) {
	flag.Parse()
	orderData := getData()
	orderCount := fstrconv.Itoa64Comma(int64(len(orderData)))
	if log {
		println(orderCount, "OrderNodes Built")
	}
	in := make(chan *msg.Message, len(orderData))
	out := make(chan *msg.Message, len(orderData))
	m := matcher.NewMatcher(*delDelay * 2)
	m.Config("Perf Matcher", in, out)
	go m.Run()
	startProfile()
	defer endProfile()
	start := time.Now().UnixNano()
	for i := range orderData {
		in <- &orderData[i]
	}
	// TODO this is only testing how fast we can push messages into the matcher, not useful
	if log {
		println("Buffer Writes: ", len(out))
		total := time.Now().UnixNano() - start
		println("Nanos\t", fstrconv.Itoa64Comma(total))
		println("Micros\t", fstrconv.Itoa64Comma(total/1000))
		println("Millis\t", fstrconv.Itoa64Comma(total/(1000*1000)))
		println("Seconds\t", fstrconv.Itoa64Comma(total/(1000*1000*1000)))
	}
}

func startProfile() {
	if *profile == "cpu" {
		f, err := os.Create("cpu.prof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}
}

func endProfile() {
	if *profile == "cpu" {
		pprof.StopCPUProfile()
	}
	if *profile == "mem" {
		f, err := os.Create("mem.prof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
	}
}

func getData() []msg.Message {
	orders, err := orderMaker.RndTradeSet(*orderNum, *delDelay, 1000, 1500)
	if err != nil {
		panic(err.Error())
	}
	return orders
}
