package clients

import (
	"fmt"
	"time"
)

type Client struct {
	Name          string
	TTL           int
	ttk           time.Duration
	lastHeartbeat time.Time
}

type Clients struct {
	clients map[string]Client
}

func NewClients() Clients {
	return Clients{
		clients: make(map[string]Client),
	}
}

func (c *Clients) Upsert(client Client) {
	client.lastHeartbeat = time.Now()
	client.ttk = time.Duration(2*client.TTL) * time.Second
	c.clients[client.Name] = client
}

func (c *Clients) Remove(client Client) {
	delete(c.clients, client.Name)
}

func (c *Clients) IsFresh(name string) (bool, error) {
	client, ok := c.clients[name]
	if !ok {
		return false, fmt.Errorf("client not found")
	}

	if time.Now().Sub(client.lastHeartbeat) < time.Duration(client.TTL)*time.Second {
		return true, nil
	}

	if time.Now().Sub(client.lastHeartbeat) > client.ttk {
		return false, fmt.Errorf("ttk is reached")
	}

	return false, nil
}
