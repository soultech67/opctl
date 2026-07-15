---
sidebar_label: Overview
title: Container Call [object]
---

An object defining a container call.

## Properties
- must have
  - [image](#image)
- may have
  - [cmd](#cmd)
  - [dirs](#dirs)
  - [envVars](#envvars)
  - [files](#files)
  - [name](#name)
  - [sockets](#sockets)
  - [volumes](#volumes)
  - [workDir](#workdir)

### image
An [image [object]](image.md) defining the container image run by the call.

### cmd
An [array](../../../../types/array.md) [initializer](../../../../types/array.md#initialization) or [variable-reference [string]](../../variable-reference.md) defining the path (from [workDir](#workdir)) of the binary to call and it's arguments.

> defining cmd overrides any entrypoint and/or cmd defined by the image

### dirs
An object for which each key is an absolute path in the container and each value is one of:

|value|meaning|
|--|--|
|null|Mount dir embedded in op w/ same path (equivalent to `$(./relative/path)`)|
|[dir](../../../../types/dir.md) [variable-reference [string]](../../variable-reference.md)|Mount dir|
|[dir initializer](../../../../types/dir.md#initialization)|Evaluate and mount|

### envVars
An [object initializer](../../../../types/object.md#initialization) or [variable-reference [string]](../../variable-reference.md), whos properties represent the name and value of an environment variable to be set in the container.

> upon evaluation, the key and value of each property will be coerced to a string.

### files
An object for which each key is an absolute path in the container and each value is one of:

|value|meaning|
|--|--|
|null|Mount file embedded in op w/ same path (equivalent to `$(./relative/path)`)|
|[file](../../../../types/file.md) [variable-reference [string]](../../variable-reference.md)|Mount file|
|[file initializer](../../../../types/file.md#initialization)|Evaluate and mount|

### name
A [string initializer](../../../../types/string.md#initialization) defining a name by which the container can be reached by other opctl containers and opctl host nodes.

> if multiple containers are given the same name, network requests will be distributed (load balanced) across them. 

### sockets
An object for which each key is an absolute path in the container and and each value is a [socket](../../../../types/socket.md) [variable-reference [string]](../../variable-reference.md) to mount. 

### volumes
An object for which each key is an absolute path in the container and each value is a [string initializer](../../../../types/string.md#initialization) naming a container-runtime-managed named volume (e.g. a Docker named volume) to mount at that path.

Unlike [dirs](#dirs) bindings, named volumes live inside the container runtime — they're never bound to a host directory, so writes don't cross the host file-sharing layer (useful for high-write-rate workloads like database data directories). They're created on first use and persist across container runs until explicitly removed (e.g. `docker volume rm <name>`).

> volume names must start with an alphanumeric character, followed by alphanumerics, `_`, `.`, or `-`

> volume names are global to the container runtime: any two containers (in the same op or different ops/projects) referencing the same name share the same volume. This makes cross-run and cross-op sharing possible, but to avoid accidental collisions use a project-unique name — [string interpolation](../../../../types/string.md#initialization) (e.g. `myproject-$(dbName)-data`) makes namespacing easy.

### workDir
A [string initializer](../../../../types/string.md#initialization) defining absolute path from which [cmd](#cmd) will be executed.

> defining workDir overrides any defined by the image