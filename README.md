# dnslb

dnslb is a dns controller which interprets healthchecks from 
[healthagent](https://github.com/NectGmbH/healthagent) and 
[healthd](https://github.com/NectGmbH/healthd) and updates dns zones depending
on whether the endpoints are reported alive or not.

dnslb is designed to be executed in an kubernetes environment,
but can be executed as standalone as well (without leaderelection)

## State

Current state is **pre-alpha** don't use it for productive infrastructure yet.

## DNS Provider

Currently only autodns is supported, but support for bind is planned as well.

## License

Licensed under [MIT](./LICENSE).