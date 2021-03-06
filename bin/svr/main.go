package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"github.com/fmstephe/matching_engine/client"
	"github.com/fmstephe/matching_engine/coordinator"
	"github.com/fmstephe/matching_engine/matcher"
	"github.com/fmstephe/matching_engine/q"
	"github.com/fmstephe/simpleid"
	"net/http"
	"os"
)

var commMaker *client.CommMaker
var idMaker = simpleid.NewIdMaker()

const (
	clientOriginId = iota
	serverOriginId = iota
)

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		println(err.Error())
		return
	}
	// Create matching engine + client
	clientToServer := q.NewSimpleQ("Client To Server")
	serverToClient := q.NewSimpleQ("Server To Client")
	// Matching Engine
	m := matcher.NewMatcher(100)
	var traderClient *client.C
	traderClient, commMaker = client.NewClient()
	coordinator.InMemory(serverToClient, clientToServer, traderClient, clientOriginId, "Client.........", true)
	coordinator.InMemory(clientToServer, serverToClient, m, serverOriginId, "Matching Engine", true)
	http.Handle("/wsconn", websocket.Handler(handleTrader))
	http.Handle("/", http.FileServer(http.Dir(pwd+"/html/")))
	if err := http.ListenAndServe("127.0.0.1:8081", nil); err != nil {
		println(err.Error())
	}
}

func handleTrader(ws *websocket.Conn) {
	traderId := uint32(idMaker.Id())
	clientComm := commMaker.NewComm(traderId)
	u := newUser(clientComm)
	orders := make(chan webMessage)
	responses := make(chan []byte)
	go reader(ws, orders)
	go writer(ws, responses)
	u.run(orders, responses)
}

func reader(ws *websocket.Conn, msgs chan<- webMessage) {
	defer close(msgs)
	defer ws.Close()
	for {
		var data string
		if err := websocket.Message.Receive(ws, &data); err != nil {
			println("error", err.Error())
			return
		}
		println(data)
		wm := &webMessage{}
		if err := json.Unmarshal([]byte(data), wm); err != nil {
			println("error", err.Error())
			return
		}
		msgs <- *wm
	}
}

func writer(ws *websocket.Conn, responses chan []byte) {
	defer ws.Close()
	for bytes := range responses {
		if _, err := ws.Write(bytes); err != nil {
			println("Writer Error", err.Error())
			return
		}
	}
}
