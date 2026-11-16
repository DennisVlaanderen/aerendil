# Aerendil

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/aerendil-logo-dark.svg"/>
    <img src="./docs/aerendil-logo.svg" width="400" alt="Aerendil"/>
  </picture>
</p>

<p align="center">
  <a href="https://github.com/DennisVlaanderen/aerendil/actions/workflows/go.yml?query=branch%3Amain"><img src="https://img.shields.io/github/actions/workflow/status/DennisVlaanderen/aerendil/go.yml?branch=main&label=backend%20build&logo=go" alt="Backend build status"/></a>
  <a href="https://github.com/DennisVlaanderen/aerendil/actions/workflows/node.js.yml?query=branch%3Amain"><img src="https://img.shields.io/github/actions/workflow/status/DennisVlaanderen/aerendil/node.js.yml?branch=main&label=ui%20build&logo=svelte" alt="UI build status"/></a>
  <a href="https://codecov.io/gh/DennisVlaanderen/aerendil"><img src="https://img.shields.io/codecov/c/github/DennisVlaanderen/aerendil/main?logo=codecov" alt="Code coverage"/></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/DennisVlaanderen/aerendil" alt="License"/></a>
  <a href="./go.mod"><img src="https://img.shields.io/github/go-mod/go-version/DennisVlaanderen/aerendil?logo=go" alt="Go version"/></a>
  <a href="https://github.com/DennisVlaanderen/aerendil/commits/main"><img src="https://img.shields.io/github/last-commit/DennisVlaanderen/aerendil/main" alt="Last commit"/></a>
  <a href="https://github.com/DennisVlaanderen"><img src="https://img.shields.io/github/followers/DennisVlaanderen?style=social" alt="GitHub followers"/></a>
  <a href="https://github.com/DennisVlaanderen/aerendil/stargazers"><img src="https://img.shields.io/github/stars/DennisVlaanderen/aerendil?style=social" alt="GitHub stars"/></a>
</p>

> [!WARNING]
> At this time Aerendil is not a working product, but very much a WIP.

Aerendil is an opinionated Feature Flagging toolkit designed from the ground
up to handle large amounts of connections and requests.
This makes Aerendil highly efficient for distributed/cloud-based workloads
running in Containers or on Kubernetes platforms.  
To accomplish this Aerendil serves flags over FQDP, a binary protocol designed
for Feature Flagging in distributed systems that runs directly over TCP rather
than over HTTP or WebSocket.

The core design principles for Aerendil are as follows:

- Developer focus should lie on developing the features, not the flagging systems
- It must support a large set of programming languages
- It must be able to handle a very large request rate for distributed workloads

## Usage

Aerendil provides a raw TCP-based feature flagging service using the FQDP
protocol. Clients can connect to query and subscribe to feature flags in
real-time.

For detailed usage examples and API documentation, please refer to the
[documentation](./docs/).

## Protocol

Aerendil exposes the FQDP (Feature Query & Definition Protocol) over TCP for
efficient feature queries and subscriptions. See the FQDP specification for
details: [docs/fqdp.md](./docs/fqdp.md).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](./CONTRIBUTING.md) for how to
set up a development environment and open a pull request.

## License

This project is licensed under the AGPL-3.0 License - see the
[LICENSE](./LICENSE) file for details.
