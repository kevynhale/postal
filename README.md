# Postal [![Circle CI](https://circleci.com/gh/jive/postal.svg?style=svg)](https://circleci.com/gh/jive/postal)

Postal is a modern IPAM service made to ease the burden of address allocation and management.

***Currently under development and not production ready.***

## Overview

With the dawn of container orchestration systems (Kubernetes, Swarm, Mesos/Marathon, etc), your choice in how to manage the underlaying network are somewhat limited.
If you want to run these systems concurrently on the same network or perhapse alongside legacy systems you're even more constrained.
Postal is an attempt to pull out a piece of this network management landscape in a way that is agnostic to the underlaying infrastructure.

Imagine you have `N` instances that sit at the edge of your network and proxy traffic to destinations within your network.
You may only have `M` public addresses available for use by these proxy instances.
You never want `N` > `M` and likewise you never want other instances to use any of the addresses that make up `M`.

Postal manages addresses as pools of resources that can grow or shrink.
As demand grows for your product, you typically add more servers to your orchestration system to increase your capacity of CPU and memory resources.
Likewise you will may add additional blocks of addresses to postal to increase your IP resources available to your infrastructure.

#### Why not NAT?

At Jive, our core business is built on VoIP which has notorious complexities traversing NAT.
Further more, NAT adds additional latency and operational burden when debugging problems.
For these reasons, NAT has no place in our network and we instead choose to build orchestration around our network management.

## Features

- Manage pools of addresses within a parent block of addresses.
- gRPC API
- CLI Tool for operator management
