# dnslb

dnslb is a dns controller which interprets healthchecks from [healthagent](https://github.com/NectGmbH/healthagent) and [healthd](https://github.com/NectGmbH/healthd) and updates dns zones depending on whether the endpoints are reported alive or not.

dnslb is designed to be be run in a fault tolerant cluster. This can be realized using its raft implementation or rely on a kubernetes e nvironment, but can be executed as standalone as well (without leaderelection)

## State

Current state is **pre-alpha** don't use it for productive infrastructure yet.

## DNS Provider

Currently only autodns is supported, but support for bind is planned as well.

## Election

The election mechanism must be specified. This allows dnslb to be deployed as a fault tolerant cluster, where only one dnslb is active at any time. This can be one of these options:

* singleton
* k8s
* raft

### Election with 'singleton'

This is used when you do not need a fault tolerant cluster.

### Election with 'k8s'

Use k8s to select the lader of the cluster.

### Election with 'raft'

Use internal raft implementation to select the lader of the cluster.

#### Setting up raft

The raft algorithum is stateful, storing the current leader and members of the cluster. Because of this it must be initialized and configured.

Raft requires each server to have a unique identity and be able to talk to all the other servers to vote on leadership and exchange state.

To provide a unique identity to a raft cluster member use the `-instance-id` argument or the `instance-identifier` configuration file option.

To provide allow comunication between raft cluster members we must provide a address. This address can be specified with the `-raft-address` argument or the `raft-address` configuration file option.

To initialize the raft cluster one and only one of the cluster nodes but be bootstrapped. This is done by running dnslb with the `-raft-bootstrap` flag.

Once this is done members of the cluster can be added. [This requires use the `raftadmin` command](https://github.com/Jille/raftadmin) which also has some documentation.


##### Example setup

This example is running 3 instances on a single node, all coomand line options excluding raft have been omitted for clarity.

 1. Make the raft state directory
    ```sh
    $ mkdir /tmp/tmpkui4bj7c
    ```
 2. Make the raft state directory for each instance
    ```sh
    $ mkdir /tmp/tmpkui4bj7c/identifier_1
    $ mkdir /tmp/tmpkui4bj7c/identifier_2
    $ mkdir /tmp/tmpkui4bj7c/identifier_3
    ```
 3. Start the first `dnslb`. Note: that we added a `-raft-bootstrap` flag.
    ```sh
    $ dnslb -election raft -instance-id identifier_1 -raft-address localhost:3648 -raft-dir /tmp/tmpkui4bj7c -raft-bootstrap
    ```
 4. Start the second and third `dnslb` instances. Note: that we did not add a `-raft-bootstrap` flag.
    ```sh
    $ dnslb -election raft -instance-id identifier_2 -raft-address localhost:3650 -raft-dir /tmp/tmpkui4bj7c
    $ dnslb -election raft -instance-id identifier_3 -raft-address localhost:3652 -raft-dir /tmp/tmpkui4bj7c
    ```
 4. Bind the `dnslb` `identifier_2` to the `dnslb` `identifier_1`.
    ```sh
    $ raftadmin localhost:3648 add_voter identifier_2 localhost:3650 0
    ```
    Now we have a two dnslb cluster with a leader.
 5. Add the final dnslb to the cluster. You can add as many as you like at this point, but it is recommended that their is an odd number.
    ```sh
    $ raftadmin --leader multi:///localhost:3648,localhost:3650 add_voter identifier_3 localhost:3652 0
    ```

##### Managing a raft cluster of `dnslb` with raft admin.

A raft cluster cannot be changed without a leader being elected.

Useful `raftadmin` commands.

0. Run with no commands to get list of available commands:
```sh
$ raftadmin
Usage: raftadmin <host:port> <command> <args...>
Commands: add_nonvoter, add_voter, applied_index, apply_log, await, barrier, demote_voter, forget, get_configuration, last_contact, last_index, leader, leadership_transfer, leadership_transfer_to_server, remove_server, shutdown, snapshot, state, stats, verify_leader
```

1. Get the raft cluster leader:
```sh
$ raftadmin --leader multi:///localhost:3648,localhost:3650,localhost:3652 leader
```
2. Change the raft cluster leader:
```sh
$ raftadmin --leader multi:///localhost:3648,localhost:3650,localhost:3652 leadership_transfer
```
2. Change the raft cluster leader to a specified leader. Note we must specify the identifier for the `dnslb` and its raft address.:
```sh
$ raftadmin --leader multi:///localhost:3648,localhost:3650,localhost:3652 leadership_transfer_to_server identifier_3 localhost:3652
```
3. Get the current raft cluster configuration.
```sh
$ raftadmin --leader multi:///localhost:3648,localhost:3650,localhost:3652 get_configuration
```

## License

Licensed under [MIT](./LICENSE).