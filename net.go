package main

import (
    "fmt"
    "net"
    "strconv"
    "strings"
)

// Endpoint represents an IP:Port tuple
type Endpoint struct {
    IP   net.IP
    Port uint16
}

// EndpointProtocol represents a tuple of an endpoint and a protocol
type EndpointProtocol struct {
    Endpoint
    Protocol string
}

func (e EndpointProtocol) String() string {
    return fmt.Sprintf("%s://%v", e.Protocol, e.Endpoint)
}

// NewEndpointProtocol creates a tuple of an endpoint and a protocol
func NewEndpointProtocol(e Endpoint, prot string) EndpointProtocol {
    return EndpointProtocol{Endpoint: e, Protocol: prot}
}

func (e Endpoint) String() string {
    return fmt.Sprintf("%s:%d", e.IP.String(), e.Port)
}

// NewEndpoint creates a new endpoint with the passed arguments.
func NewEndpoint(ip net.IP, port uint16) Endpoint {
    return Endpoint{
        IP:   ip,
        Port: port,
    }
}

// TryParseEndpointProtocols tries to parse a list of protocol-endpoint tuple like "tcp://ip1:port1,tcp://ip2:port2"
func TryParseEndpointProtocols(str string) ([]EndpointProtocol, error) {
    splitted := strings.Split(str, ",")
    eps := make([]EndpointProtocol, 0)

    for _, s := range splitted {
        ep, err := TryParseEndpointProtocol(s)
        if err != nil {
            return nil, fmt.Errorf("couldn't parse `%s` as EndpointProtocol, see: %v", s, err)
        }

        eps = append(eps, ep)
    }

    return eps, nil
}

// TryParseEndpointProtocol tries to parse a protocol-endpoint tuple like "tcp://ip:port"
func TryParseEndpointProtocol(str string) (EndpointProtocol, error) {
    splitted := strings.Split(str, "://")
    if len(splitted) != 2 {
        return EndpointProtocol{}, fmt.Errorf("expected string in format schema://ip:port but got `%s`", str)
    }

    prot := splitted[0]

    endpoint, err := TryParseEndpoint(splitted[1])
    if err != nil {
        return EndpointProtocol{}, fmt.Errorf("couldn't parse endpoint from `%s`, see: %v", splitted[1], err)
    }

    return NewEndpointProtocol(endpoint, prot), nil
}

// TryParseEndpoint tries to parse to passed string in the format ip:port as endpoint
func TryParseEndpoint(str string) (Endpoint, error) {
    splitted := strings.Split(str, ":")
    if len(splitted) != 2 {
        return Endpoint{}, fmt.Errorf("expected ip:port but got `%s`", str)
    }

    ip := net.ParseIP(splitted[0]).To4()
    port, err := strconv.Atoi(splitted[1])
    if err != nil {
        return Endpoint{}, fmt.Errorf("couldnt parse port, see: %v", err)
    }

    return NewEndpoint(ip, uint16(port)), nil
}
