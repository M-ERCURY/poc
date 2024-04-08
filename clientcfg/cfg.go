// Package clientcfg describes the configuration file format and data types
// used by mercury.
package clientcfg

import (
	"net/url"
	"time"

	"github.com/M-ERCURY/core/api/duration"
	"github.com/M-ERCURY/core/api/texturl"
)

// C is the type of the config struct describing the config file format.
type C struct {
	// Timeout is the dial timeout.
	Timeout duration.T `json:"timeout,omitempty"`
	// Contract is the service contract used by this mercury.
	Contract *texturl.URL `json:"contract,omitempty"`
	// Accesskey is the section dealing with accesskey configuration.
	Accesskey Accesskey `json:"accesskey,omitempty"`
	// Circuit describes the configuration of the mercury connection circuit.
	Circuit Circuit `json:"circuit,omitempty"`
	// Address describes the listening addresses and ports.
	Address Address `json:"address,omitempty"`

	PofURL string `json:"pof_url,omitempty"`
}

// Accesskey is the section dealing with accesskey configuration.
type Accesskey struct {
	// UseOnDemand sets whether pofs are used to generate new servicekeys
	// automatically.
	UseOnDemand bool `json:"use_on_demand,omitempty"`
}

// Circuit describes the configuration of the Mercury connection circuit.
type Circuit struct {
	// Whitelist is the optional user-defined list of relays to use exclusively.
	Whitelist *[]string `json:"whitelist,omitempty"`
	// Hops is the desired number of hops to use for the circuit.
	Hops int `json:"hops,omitempty"`
}

// Address describes the listening addresses and ports.
type Address struct {
	// Address.Socks is the SOCKSv5 TCP and UDP listening address.
	Socks *string `json:"socks,omitempty"`
	// Address.H2C is the h2c listening address for local connections.
	H2C *string `json:"h2c,omitempty"`
	// Address.Tun is the listening address configuration for mercury_tun.
	Tun *string `json:"tun,omitempty"`
}

// Defaults provides a config with sane defaults whenever possible.
func Defaults() C {
	var (
		sksaddr = "127.0.0.1:13491"
		h2caddr = "127.0.0.1:13492"
		tunaddr = "10.13.49.0:13493"
	)
	contractURL, _ := url.Parse("http://34.133.212.204:3001")

	return C{
		Contract: &texturl.URL{
			URL: *contractURL,
		},
		PofURL:    "http://34.133.212.204:3003/buy?quantity=1",
		Accesskey: Accesskey{UseOnDemand: true},
		Timeout:   duration.T(time.Second * 5),
		Circuit:   Circuit{Hops: 1},
		Address: Address{
			Socks: &sksaddr,
			H2C:   &h2caddr,
			Tun:   &tunaddr,
		},
	}
}

type Meta struct {
	// Option name
	Name string
	// Name of the "type'
	Type string
	// Description
	Desc string
	// Pointer to value to feed to Unmarshal()
	Val interface{}
	// Whether the input needs to be quoted before calling Unmarshal()
	Quote bool
}

func (c *C) Metadata() []*Meta {
	return []*Meta{
		{"timeout", "str", "Dial timeout duration", &c.Timeout, true},
		{"contract", "str", "Service contract associated with accesskeys", &c.Contract, true},
		{"address.socks", "str", "SOCKS5 proxy address of mercury daemon", &c.Address.Socks, true},
		{"address.h2c", "str", "H2C proxy address of mercury daemon", &c.Address.H2C, true},
		{"address.tun", "str", "TUN device address (not loopback)", &c.Address.Tun, true},
		{"circuit.hops", "int", "Number of relay hops to use in a circuit", &c.Circuit.Hops, false},
		{"circuit.whitelist", "list", "Whitelist of relays to use", &c.Circuit.Whitelist, false},
		{"accesskey.use_on_demand", "bool", "Activate accesskeys as needed", &c.Accesskey.UseOnDemand, false},
	}
}
