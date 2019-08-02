package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    etcdClient "go.etcd.io/etcd/client"
)

// ETCDProvider is an generic abstraction over the etcd api
type ETCDProvider interface {
    // Init initializes the etcd provider.
    Init(endpoints []string) error

    // Set sets the value for a specific key.
    Set(key string, value string) error

    // Get retrieves the value for a specific key.
    Get(key string) (string, error)
}

// ETCD represents the etcd provider
type ETCD struct {
    client etcdClient.Client
}

// Init initializes the etcd provider.
func (e *ETCD) Init(endpoints []string) error {
    var err error
    e.client, err = etcdClient.New(etcdClient.Config{
        Endpoints: endpoints,
    })

    return err
}

// Set sets the value for a specific key.
func (e *ETCD) Set(key string, value string) error {
    kapi := etcdClient.NewKeysAPI(e.client)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    _, err := kapi.Set(ctx, key, value, nil)
    cancel()

    return err
}

// Get retrieves the value for a specific key.
func (e *ETCD) Get(key string) (string, error) {
    kapi := etcdClient.NewKeysAPI(e.client)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    resp, err := kapi.Get(ctx, key, nil)
    cancel()

    if err != nil {
        return "", err
    } else if resp == nil || resp.Node == nil {
        return "", fmt.Errorf("missing node in response from etcd")
    }

    return resp.Node.Value, nil
}

// ETCDMock is a mocked etcd server
type ETCDMock struct {
    state map[string]string
    lock  sync.Mutex
}

// Init initializes the etcd provider.
func (e *ETCDMock) Init(endpoints []string) error {
    e.state = make(map[string]string)

    return nil
}

// Set sets the value for a specific key.
func (e *ETCDMock) Set(key string, value string) error {
    e.lock.Lock()
    defer e.lock.Unlock()

    e.state[key] = value

    return nil
}

// Get retrieves the value for a specific key.
func (e *ETCDMock) Get(key string) (string, error) {
    e.lock.Lock()
    defer e.lock.Unlock()

    val, ok := e.state[key]
    if !ok {
        return "", fmt.Errorf("no value for key `%s` found", key)
    }

    return val, nil
}
