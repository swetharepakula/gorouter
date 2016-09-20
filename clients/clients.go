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
	fmt.Printf("Upserting client: %#v\n", client)
	client.lastHeartbeat = time.Now()
	client.ttk = time.Duration(3*client.TTL) * time.Second
	c.clients[client.Name] = client
}

func (c *Clients) Remove(client Client) {
	fmt.Printf("Removing client: %#v\n", client)
	delete(c.clients, client.Name)
}

func (c *Clients) IsFresh(name string) (bool, error) {
	client, ok := c.clients[name]
	if !ok {
		fmt.Printf("client %s not found in map: %#v\n", name, c.clients)
		return false, fmt.Errorf("client not found")
	}

	if time.Now().Sub(client.lastHeartbeat) < time.Duration(client.TTL)*time.Second {
		fmt.Printf("client is fresh: %#v\n", client)
		return true, nil
	}

	if time.Now().Sub(client.lastHeartbeat) > client.ttk {
		fmt.Printf("client has reached TTK: %#v\n", client)
		return false, fmt.Errorf("ttk is reached")
	}

	fmt.Printf("client is not fresh: %#v\n", client)
	return false, nil
}
