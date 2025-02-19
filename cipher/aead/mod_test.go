package aead

import (
	"crypto/sha512"
	"encoding/hex"
	"github.com/Yayg/noise"
	"github.com/Yayg/noise/log"
	"github.com/Yayg/noise/protocol"
	"github.com/Yayg/noise/transport"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var transportLayer = transport.NewBuffered()

func node(t *testing.T) *noise.Node {
	params := noise.DefaultParams()
	params.Transport = transportLayer

	params.ReceiveMessageTimeout = 10 * time.Millisecond
	params.SendMessageTimeout = 10 * time.Millisecond
	params.SendWorkerBusyTimeout = 10 * time.Millisecond

	node, err := noise.NewNode(params)
	assert.NoError(t, err)

	go node.Listen()

	return node
}

type msg struct{ noise.EmptyMessage }

var _ protocol.Block = (*receiverBlock)(nil)

type receiverBlock struct {
	opcodeMsg noise.Opcode
	receiver  chan interface{}
}

func (b *receiverBlock) OnRegister(p *protocol.Protocol, node *noise.Node) {
	b.opcodeMsg = noise.RegisterMessage(noise.NextAvailableOpcode(), (*msg)(nil))
	b.receiver = make(chan interface{}, 1)
}

func (b *receiverBlock) OnBegin(p *protocol.Protocol, peer *noise.Peer) error {
	err := peer.SendMessage(msg{})
	if err != nil {
		return err
	}

	b.receiver <- <-peer.Receive(b.opcodeMsg)
	return nil
}

func (b *receiverBlock) OnEnd(p *protocol.Protocol, peer *noise.Peer) error {
	return nil
}

func TestBlock_OnBeginEdgeCases(t *testing.T) {
	log.Disable()
	defer log.Enable()

	// Setup Alice and Bob.
	alice, bob := node(t), node(t)

	defer alice.Kill()
	defer bob.Kill()

	// Enforce protocols.
	block := New().WithHash(sha512.New).WithACKTimeout(1 * time.Millisecond)
	proto := protocol.New().Register(block)

	proto.Enforce(alice)
	proto.Enforce(bob)

	// Have Alice dial Bob.
	peerBob1, err := alice.Dial(bob.ExternalAddress())
	assert.NoError(t, err)
	defer peerBob1.Disconnect()

	// Check opcode is registered.
	assert.True(t, block.opcodeACK != noise.OpcodeNil)

	// Expect a disconnect calling OnBegin without Bob yet having an ephemeral shared key.
	assert.True(t, errors.Cause(block.OnBegin(proto, peerBob1)) == protocol.DisconnectPeer)

	// Now restart connections, and set a proper ephemeral shared key to Bob.
	ephemeralSharedKey, err := hex.DecodeString("d8747263b4d54588c2c8f17862d827dee6d3893a02fb7a84800b001ad4f1cee8")
	assert.NoError(t, err)

	peerBob3, err := alice.Dial(bob.ExternalAddress())
	assert.NoError(t, err)
	defer peerBob3.Disconnect()

	protocol.SetSharedKey(peerBob3, ephemeralSharedKey)
	assert.True(t, errors.Cause(block.OnBegin(proto, peerBob3)) == protocol.DisconnectPeer)
}

func TestBlock_OnBeginSuccessful(t *testing.T) {
	log.Disable()
	defer log.Enable()

	ephemeralSharedKey, err := hex.DecodeString("d8747263b4d54588c2c8f17862d827dee6d3893a02fb7a84800b001ad4f1cee8")
	assert.NoError(t, err)

	alice, bob := node(t), node(t)

	defer alice.Kill()
	defer bob.Kill()

	alice.OnPeerInit(func(node *noise.Node, peer *noise.Peer) error {
		peer.Set(protocol.KeySharedKey, ephemeralSharedKey)
		return nil
	})

	bob.OnPeerInit(func(node *noise.Node, peer *noise.Peer) error {
		peer.Set(protocol.KeySharedKey, ephemeralSharedKey)
		return nil
	})

	aliceReceiver, bobReceiver := new(receiverBlock), new(receiverBlock)

	protocol.New().Register(New()).Register(aliceReceiver).Enforce(alice)
	protocol.New().Register(New()).Register(bobReceiver).Enforce(bob)

	peer, err := alice.Dial(bob.ExternalAddress())
	assert.NoError(t, err)
	defer peer.Disconnect()

	<-aliceReceiver.receiver
	<-bobReceiver.receiver
}
