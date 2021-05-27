package main

/* Based upon https://github.com/Jille/raft-grpc-example.git
 */

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
	transport "github.com/Jille/raft-grpc-transport"
	"github.com/Jille/raftadmin"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RaftController represents a raft controller
type RaftController struct {
	raft             *raft.Raft
	transportManager *transport.Manager
	running          bool
	grpcServer       *grpc.Server
	socket           *net.Listener
}

// NewRaftController creates a new controller
func NewRaftController(myAddr string, raftID string, raftDir string, raftBootstrap bool) *RaftController {

	ctx := context.Background()
	_, port, err := net.SplitHostPort(myAddr)
	if err != nil {
		log.Fatalf("failed to parse local address (%q): %v", myAddr, err)
	}
	sock, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	wt := &replicatedState{}

	r, tm, err := newRaft(ctx, raftID, myAddr, wt, raftDir, raftBootstrap)
	if err != nil {
		log.Fatalf("failed to start raft: %v", err)
	}
	s := grpc.NewServer()
	tm.Register(s)
	leaderhealth.Setup(r, s, []string{"Example"})
	raftadmin.Register(s, r)
	reflection.Register(s)

	ctrl := &RaftController{
		raft:             r,
		transportManager: tm,
		grpcServer:       s,
		running:          false,
		socket:           &sock,
	}

	return ctrl

}

// Run starts the controller in a async, non blocking way
func (c *RaftController) Run() {
	if c.running {
		return
	}
	if err := c.grpcServer.Serve(*c.socket); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Stop the controller in a async, non blocking way
func (c *RaftController) Stop() {
	if !c.running {
		return
	}
	c.grpcServer.Stop()
}

// LeaderCh returns the leader channel.
func (c *RaftController) LeaderCh() <-chan bool {
	return c.raft.LeaderCh()
}

func newRaft(ctx context.Context, myID, myAddress string, fsm raft.FSM, raftDir string, raftBootstrap bool) (*raft.Raft, *transport.Manager, error) {
	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(myID)

	baseDir := filepath.Join(raftDir, myID)

	ldb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "logs.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "logs.dat"), err)
	}

	sdb, err := boltdb.NewBoltStore(filepath.Join(baseDir, "stable.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "stable.dat"), err)
	}

	fss, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q, ...): %v`, baseDir, err)
	}

	tm := transport.New(raft.ServerAddress(myAddress), []grpc.DialOption{grpc.WithInsecure()})

	r, err := raft.NewRaft(c, fsm, ldb, sdb, fss, tm.Transport())
	if err != nil {
		return nil, nil, fmt.Errorf("raft.NewRaft: %v", err)
	}

	if raftBootstrap {
		cfg := raft.Configuration{
			Servers: []raft.Server{
				{
					Suffrage: raft.Voter,
					ID:       raft.ServerID(myID),
					Address:  raft.ServerAddress(myAddress),
				},
			},
		}
		f := r.BootstrapCluster(cfg)
		if err := f.Error(); err != nil {
			return nil, nil, fmt.Errorf("raft.Raft.BootstrapCluster: %v", err)
		}
	}

	return r, tm, nil
}
