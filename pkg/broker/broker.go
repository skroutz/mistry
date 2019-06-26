package broker

import (
	"bufio"
	"log"
	"sync"

	"github.com/skroutz/mistry/pkg/tailer"
)

// A Broker holds a registry with open client connections, listens for events on the
// Notifier channel and broadcasts event messages to the corresponding clients.
type Broker struct {
	Log *log.Logger

	// Messages are pushed to this channel.
	Notifier chan *Event

	// Channel for adding new client connections.
	NewClients chan *Client

	// Channel for signaling a closed client connection.
	ClosingClients chan *Client

	// Channel for signaling the closing of all connections for an id.
	CloseClientC map[string]chan struct{}

	// clients is the connections registry of the Broker. clients sent to the
	// NewClients channel are being added to the registry.
	// A reference to the Client is being used so that the connections can be
	// uniquely identified for the messages broadcasting.
	clients map[*Client]bool

	// Queue used to track all open clients count grouped by their id.
	// The stored map type is [string]int.
	clientsCount *sync.Map
}

// Client represents a client-connection.
type Client struct {
	// The connection channel to communicate with the events gathering
	// channel.
	Data chan []byte

	// Each connection has an id that corresponds to the Event ID it is
	// interested in receiving messages about.
	ID string

	// Extra contains any extra misc information about the connection.
	// e.g a secondary unique identifier for the Client
	Extra string
}

// Event consists of an id ID and a message Msg. All clients with the same id
// receive the event message.
type Event struct {
	// The message to be consumed by any connected client e.g., browser.
	Msg []byte

	// Each message has an id which corresponds to the concerning client id.
	ID string
}

// NewBroker returns a new Broker.
func NewBroker(logger *log.Logger) *Broker {
	br := &Broker{}
	br.Log = logger
	br.Notifier = make(chan *Event)
	br.NewClients = make(chan *Client)
	br.ClosingClients = make(chan *Client)
	br.clients = make(map[*Client]bool)
	br.clientsCount = new(sync.Map)
	br.CloseClientC = make(map[string]chan struct{})
	return br
}

// ListenForClients is responsible for taking the appropriate course of
// action based on the different channel messages. It listens for new clients
// on the NewClients channel, for closing clients on the ClosingClients channel
// and for events Event on the Notifier channel.
func (br *Broker) ListenForClients() {
	for {
		select {
		case client := <-br.NewClients:
			br.clients[client] = true
			val, exists := br.clientsCount.Load(client.ID)
			cc, ok := val.(int)
			if exists && !ok {
				br.Log.Printf("got data of type %T but wanted int", val)
			}
			if exists && cc > 0 {
				br.clientsCount.Store(client.ID, cc+1)
			} else {
				br.clientsCount.Store(client.ID, 1)
				br.CloseClientC[client.ID] = make(chan struct{})
				tl, err := tailer.New(client.Extra)
				if err != nil {
					br.Log.Printf("[broker] Could not start the tailer for file %s", client.Extra)
				}
				go func() {
					s := bufio.NewScanner(tl)
					for s.Scan() {
						br.Notifier <- &Event{Msg: []byte(s.Text()), ID: client.ID}
					}
				}()
				go func() {
					<-br.CloseClientC[client.ID]
					err = tl.Close()
					if err != nil {
						br.Log.Print(err)
					}
				}()
			}
		case client := <-br.ClosingClients:
			close(client.Data)
			delete(br.clients, client)
			val, _ := br.clientsCount.Load(client.ID)
			cc, ok := val.(int)
			if !ok {
				br.Log.Printf("got data of type %T but wanted int", val)
			}
			newVal := cc - 1
			br.clientsCount.Store(client.ID, newVal)
			if newVal == 0 {
				br.CloseClientC[client.ID] <- struct{}{}
			}
		case event := <-br.Notifier:
			for client := range br.clients {
				if client.ID == event.ID {
					client.Data <- event.Msg
				}
			}
		}
	}
}
