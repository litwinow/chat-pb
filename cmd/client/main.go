package main

import (
	"bufio"
	"chat-pb/pkg/protocol"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

var username = flag.String("u", "", fmt.Sprintf("username (%d chars)", protocol.UsernameLength))
var throttle = flag.Uint64("n", 0, "get messages if n of them are pending")
var serverAddr = flag.String("h", "", "server address")

func main() {
	flag.Parse()
	if len([]byte(*username)) != protocol.UsernameLength || len(*serverAddr) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	conn, err := setupConnection(*serverAddr)
	if err != nil {
		return err
	}

	id, err := subscribe(conn, *username, *throttle)
	if err != nil {
		return err
	}

	go readMessages(conn)

	return sendMessages(conn, id)
}

func setupConnection(serverAddr string) (*net.UDPConn, error) {
	const network = "udp"
	udpAddr, err := net.ResolveUDPAddr(network, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("couldn't resolve udp addr: %v", err)
	}

	conn, err := net.DialUDP(network, nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("couldn't dial udp: %v", err)
	}

	return conn, nil
}

func subscribe(conn *net.UDPConn, username string, throttle uint64) (uint64, error) {
	if err := sendInitRequest(conn, username, throttle); err != nil {
		return 0, fmt.Errorf("failed to send init request: %v", err)
	}

	return readInitResponse(conn)
}

func sendInitRequest(conn *net.UDPConn, username string, throttle uint64) error {
	buf := make([]byte, 14)
	buf[0] = protocol.InitReq
	copy(buf[1:6], []byte(username))
	binary.BigEndian.PutUint64(buf[6:14], throttle)

	_, err := conn.Write(buf)
	return err
}

func readInitResponse(conn *net.UDPConn) (uint64, error) {
	buf := make([]byte, 9)
	_, err := conn.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("failed to read from conn: %v", err)
	}
	if buf[0] != protocol.InitResp {
		return 0, fmt.Errorf("incorrect resp type: %d", buf[0])
	}
	return binary.BigEndian.Uint64(buf[1:9]), nil
}

func sendMessage(conn *net.UDPConn, id uint64, text string) error {
	buf := make([]byte, protocol.MaxMessageLength+9)
	buf[0] = protocol.Message
	binary.BigEndian.PutUint64(buf[1:9], id)
	copy(buf[9:], []byte(text))
	_, err := conn.Write(buf)
	return err
}

func readMessages(conn *net.UDPConn) {
	buf := make([]byte, protocol.MaxMessageLength)
	for {
		n, err := conn.Read(buf)
		if err == nil {
			fmt.Println(string(buf[0:n]))
		}
	}
}

func sendMessages(conn *net.UDPConn, id uint64) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimRight(text, "\n")
		if err := sendMessage(conn, id, text); err != nil {
			return err
		}
	}
}
