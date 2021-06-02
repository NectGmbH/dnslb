package main

import (
	"time"

	"github.com/sirupsen/logrus"
	bully "github.com/timtosi/bully-algorithm"
)

// BullyController represents a bully controller
type BullyController struct {
	bully         *bully.Bully
	leadingStarts *time.Time
	leaderMsgLast bool
	leader        chan bool
	keepRuning    bool
}

// NewBullyController makes a new instance
func NewBullyController(ID, addr, proto string, peers map[string]string) (*BullyController, error) {
	logrus.WithFields(logrus.Fields{
		"peers": peers,
		"proto": proto,
		"addr":  addr,
		"ID":    ID,
	}).Debug("Setup Bully Controller")
	var bullyPtr, err = bully.NewBully(ID, addr, proto, peers)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Debug("NewBullyController")
		return nil, err
	}
	var bc = &BullyController{
		bully:         bullyPtr,
		leaderMsgLast: false,
		leader:        make(chan bool),
	}
	return bc, nil
}

func (c *BullyController) handleVoter() {
	if c.leadingStarts == nil {
		return
	}
	c.leadingStarts = nil
	if c.leaderMsgLast == false {
		return
	}
	go func() { c.leader <- false }()
	c.leaderMsgLast = false
	logrus.Debug("Sent not leader message")
}

func (c *BullyController) handleCoordinator() {
	if c.leaderMsgLast == true {
		return
	}
	if c.leadingStarts == nil {
		deadline := time.Now().Add(time.Second * 10)
		c.leadingStarts = &deadline
		return
	}
	if time.Now().Before(*c.leadingStarts) {
		return
	}
	go func() { c.leader <- true }()
	c.leaderMsgLast = true
	logrus.Debug("Sent leader message")
}

func (c *BullyController) workFunc() {
	for c.keepRuning {
		if c.bully.ID == c.bully.Coordinator() {
			c.handleCoordinator()
		} else {
			c.handleVoter()
		}
		time.Sleep(1 * time.Second)
	}
}

// Run starts the controller in a async, non blocking way
func (c *BullyController) Run() {
	logrus.Debug("run")
	c.keepRuning = true
	c.bully.Run(c.workFunc)
}

// LeaderCh returns the leader channel.
func (c *BullyController) LeaderCh() <-chan bool {
	return c.leader
}

// Stop the controller in a async, non blocking way
func (c *BullyController) Stop() {
	c.keepRuning = false
}
