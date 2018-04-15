package broker

import (
	"log"
	"sync"
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

	// Clients is the connections registry of the Broker. Clients sent to the
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
	EventC chan []byte

	// Each connection has an id that corresponds to the Event ID it is
	// interested in receiving messages about.
	ID string
}

// Event consists of an id ID and a message Msg. All clients with the same id
// receive the event message.
type Event struct {
	// The message to be consumed by any connected client e.g., browser.
	Msg []byte

	// Each message has an id which corresponds to the concerning client id.
	ID string
}

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
		case sb := <-br.NewClients:
			br.clients[sb] = true
			cc, ok := br.clientsCount.Load(sb.ID)
			if ok {
				br.clientsCount.Store(sb.ID, cc.(int)+1)
			} else {
				br.clientsCount.Store(sb.ID, 1)
			}
			br.Log.Printf("[broker] Client added. %d registered clients", len(br.clients))
		case sb := <-br.ClosingClients:
			delete(br.clients, sb)
			cc, ok := br.clientsCount.Load(sb.ID)
			if ok {
				br.clientsCount.Store(sb.ID, cc.(int)-1)
			} else {
				br.clientsCount.Store(sb.ID, 0)
			}
			br.Log.Printf("[broker] Removed client. %d registered clients", len(br.clients))
			cc, ok = br.clientsCount.Load(sb.ID)
			if ok && cc.(int) == 0 {
				br.CloseClientC[sb.ID] <- struct{}{}
			}
		case event := <-br.Notifier:
			for client, _ := range br.clients {
				if client.ID == event.ID {
					client.EventC <- event.Msg
				}
			}
		}
	}
}
