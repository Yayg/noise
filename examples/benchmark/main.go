package main

import (
	"crypto/rand"
	"fmt"
	"github.com/Yayg/noise"
	"github.com/Yayg/noise/cipher/aead"
	"github.com/Yayg/noise/handshake/ecdh"
	"github.com/Yayg/noise/log"
	"github.com/Yayg/noise/payload"
	"github.com/Yayg/noise/protocol"
	"github.com/Yayg/noise/skademlia"
	"github.com/pkg/errors"
	"sync/atomic"
	"time"
)

var (
	_ noise.Message = (*benchmarkMessage)(nil)

	messagesSentPerSecond     uint64
	messagesReceivedPerSecond uint64
)

type benchmarkMessage struct {
	text string
}

func (benchmarkMessage) Read(reader payload.Reader) (noise.Message, error) {
	text, err := reader.ReadString()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read msg")
	}

	return benchmarkMessage{text: text}, nil
}

func (m benchmarkMessage) Write() []byte {
	return payload.NewWriter(nil).WriteString(m.text).Bytes()
}

func spawnNode(port uint16) *noise.Node {
	params := noise.DefaultParams()
	params.Keys = skademlia.RandomKeys()
	params.Port = port

	node, err := noise.NewNode(params)
	if err != nil {
		panic(err)
	}

	p := protocol.New()
	p.Register(ecdh.New())
	p.Register(aead.New())
	p.Register(skademlia.New())
	p.Enforce(node)

	go node.Listen()

	log.Info().Msgf("Listening for peers on port %d.", node.ExternalPort())

	return node
}

func main() {
	opcodeBenchmark := noise.RegisterMessage(noise.NextAvailableOpcode(), (*benchmarkMessage)(nil))

	server, client := spawnNode(0), spawnNode(0)

	go func() {
		for range time.Tick(3 * time.Second) {
			sent, received := atomic.SwapUint64(&messagesSentPerSecond, 0), atomic.SwapUint64(&messagesReceivedPerSecond, 0)

			fmt.Printf("Sent %d, and received %d messages per second.\n", sent, received)
		}
	}()

	server.OnPeerConnected(func(node *noise.Node, peer *noise.Peer) error {
		go func() {
			aead.WaitUntilAuthenticated(peer)

			for {
				payload := make([]byte, 600)
				_, _ = rand.Read(payload)

				atomic.AddUint64(&messagesSentPerSecond, 1)
				_ = peer.SendMessageAsync(benchmarkMessage{text: string(payload)})
			}
		}()

		return nil
	})

	client.OnPeerDialed(func(node *noise.Node, peer *noise.Peer) error {
		go func() {
			for {
				<-peer.Receive(opcodeBenchmark)
				atomic.AddUint64(&messagesReceivedPerSecond, 1)
			}
		}()

		return nil
	})

	_, err := client.Dial(server.ExternalAddress())
	if err != nil {
		panic(err)
	}

	select {}
}
