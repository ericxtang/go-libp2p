package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	net "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	pstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	bhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/basic"
	host "gx/ipfs/Qmc1XhrFEiSeBNn3mpfg6gEuYCt5im2gYmNVmncsvmpeAk/go-libp2p-host"
	"time"

	"github.com/golang/glog"
	mcjson "github.com/multiformats/go-multicodec/json"
)

var Protocol = protocol.ID("/test_protocol/0.0.1")

func main() {
	// race1()
	race2()
}

func race1() {
	h1 := makeHost(15000)
	h2 := makeHost(15001)
	connectHosts(h1, h2)

	//Handle messages on h2, just put it into a channel
	c := make(chan string)
	h2.SetStreamHandler(Protocol, func(stream net.Stream) {
		for {
			bytes := make([]byte, 500)
			if _, err := stream.Read(bytes); err != nil {
				fmt.Printf("Error: %v", err)
				break
			}
			c <- string(bytes)
		}
	})

	//Get a stream from h1 to h2
	strm, err := h1.NewStream(context.Background(), h2.ID(), Protocol)
	if err != nil {
		fmt.Printf("Error creating stream: %v", err)
		return
	}

	//Send 20 messages on the stream
	go func() {
		for i := 0; i < 20; i++ {
			strm.Write([]byte(fmt.Sprintf("%v", i)))
		}
	}()

	//Monitor the channel, we should expect 20 messages
	for i := 0; i < 20; i++ {
		select {
		case i := <-c:
			fmt.Println("Got message: " + i)
		case <-time.After(2 * time.Second):
			fmt.Printf("Error: Timed Out!\n")
		}

	}
}

type Msg struct {
	Name string
	Data []byte
}

func race2() {
	h1 := makeHost(15000)
	h2 := makeHost(15001)
	connectHosts(h1, h2)

	//Handle messages on h2, unmarshal the message and parse out the Name.
	c := make(chan string)
	h2.SetStreamHandler(Protocol, func(stream net.Stream) {
		defer stream.Reset()
		reader := bufio.NewReader(stream)
		dec := mcjson.Multicodec(false).Decoder(reader)

		for {
			var msg Msg
			if err := dec.Decode(&msg); err != nil {
				fmt.Printf("Error: %v", err)
				break
			}
			c <- string(msg.Name)
		}
	})

	//Get a stream from h1 to h2
	strm, err := h1.NewStream(context.Background(), h2.ID(), Protocol)
	if err != nil {
		fmt.Printf("Error creating stream: %v", err)
		return
	}
	writer := bufio.NewWriter(strm)
	enc := mcjson.Multicodec(false).Encoder(writer)

	//Send 20 messages on the stream
	go func() {
		for i := 0; i < 20; i++ {
			enc.Encode(Msg{Name: fmt.Sprintf("%v", i)})
			writer.Flush()
		}
	}()

	//Monitor the channel, we should expect 20 messages
	for i := 0; i < 20; i++ {
		select {
		case i := <-c:
			fmt.Println("Got message: " + i)
		case <-time.After(2 * time.Second):
			fmt.Printf("Error: Timed Out!\n")
		}
	}
}

func connectHosts(h1, h2 host.Host) {
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), pstore.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), pstore.PermanentAddrTTL)
	err := h1.Connect(context.Background(), pstore.PeerInfo{ID: h2.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h1 with h2: %v", err)
	}
	err = h2.Connect(context.Background(), pstore.PeerInfo{ID: h1.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h2 with h1: %v", err)
	}

	time.Sleep(time.Millisecond * 50)
}

func makeHost(port int) host.Host {
	// Generate an identity keypair using go's cryptographic randomness source
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	// A peers ID is the hash of its public key
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}

	// We've created the identity, now we need to store it.
	// A peerstore holds information about peers, including your own
	ps := pstore.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)

	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", port))
	if err != nil {
		panic(err)
	}

	// Make a context to govern the lifespan of the swarm
	ctx := context.Background()

	// Put all this together
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{maddr}, pid, ps, nil)
	if err != nil {
		panic(err)
	}

	return bhost.New(netw)
}
