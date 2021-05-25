package main

import (
	"bytes"
	"chat-pb/pkg/protocol"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
)

type client struct {
	addr            *net.UDPAddr
	name            string
	throttle        uint64
	pendingMessages []*message
}

type message struct {
	senderID   uint64
	senderName string
	text       string
}

var listeners = make(map[uint64]*client)
var userID uint64
var msgC = make(chan message)

var port = flag.Int("p", 9001, "server port")

func main() {
	flag.Parse()

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v", err)
		os.Exit(1)
	}

	go broadcastMessages(conn)

	for {
		handleClient(conn)
	}
}

func handleClient(conn *net.UDPConn) {
	buf := make([]byte, 512)

	_, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return
	}
	switch buf[0] {
	case protocol.InitReq:
		username := string(buf[1:6])
		throttle := binary.BigEndian.Uint64(buf[6:14])
		sendInitResponse(conn, addr, username, throttle)
	case protocol.Message:
		senderID := binary.BigEndian.Uint64(buf[1:9])
		handleMsg(conn, senderID, bytes.Trim(buf[9:], "\x00"))
	}
}

func handleMsg(conn *net.UDPConn, senderID uint64, text []byte) {
	if len(string(text)) == 0 {
		return
	}
	sender, exists := listeners[senderID]
	if !exists {
		return
	}
	msgC <- message{
		senderID:   senderID,
		senderName: sender.name,
		text:       string(text),
	}
}

func sendInitResponse(conn *net.UDPConn, sender *net.UDPAddr, username string, throttle uint64) {
	if throttle == 0 {
		throttle = 1
	}
	buf := make([]byte, 9)
	buf[0] = protocol.InitResp
	binary.BigEndian.PutUint64(buf[1:9], userID)
	listeners[userID] = &client{
		addr:            sender,
		name:            username,
		throttle:        throttle,
		pendingMessages: make([]*message, 0, throttle),
	}
	userID++
	conn.WriteToUDP(buf, sender)
}

func broadcastMessages(conn *net.UDPConn) {
	for {
		msg := <-msgC
		for userID, user := range listeners {
			if msg.senderID == userID {
				continue
			}
			user.pendingMessages = append(user.pendingMessages, &msg)
			if len(user.pendingMessages) == int(user.throttle) {
				for _, m := range user.pendingMessages {
					conn.WriteToUDP([]byte(fmt.Sprintf("%s: %s", m.senderName, m.text)), user.addr)
				}
				user.pendingMessages = make([]*message, 0, user.throttle)
			}
		}
	}
}
