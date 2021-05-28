package main

/*
 Simplest and smallest possible state wrapper.
*/

import (
	"io"

	"github.com/hashicorp/raft"
)

// replicatedState
type replicatedState struct {
}

var _ raft.FSM = &replicatedState{}

func (f *replicatedState) Apply(l *raft.Log) interface{} {
	return nil
}

func (f *replicatedState) Snapshot() (raft.FSMSnapshot, error) {
	// Make sure that any future calls to f.Apply() don't change the snapshot.
	return &snapshot{}, nil
}

func (f *replicatedState) Restore(r io.ReadCloser) error {
	return nil
}

type snapshot struct {
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	return sink.Close()
}

func (s *snapshot) Release() {
}
