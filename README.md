# regsync

## Objective
Develop code to make it easier/less resource intensive to maintain a set of images in sync from a "master" registry to a set of "replica" registries.

Replicas could potentially be many, geographically distributed, and with limited available bandwidth to the master.

## Status

An ongoing [DESIGN](DESIGN.md) document and tools for experimentation of ideas.

## Current code status

A commandline tool to experiment:

```
NAME:
   regsync - Utility to synchronize container image registries

USAGE:
   regsync [global options] command [command options] [arguments...]

VERSION:
   0.1

COMMANDS:
   compress        compresses a file with go's gzip to standard output
   decompress      decompresses a file with go's gzip to standard output
   isgzip          Exists with 0 if input file is gzipped
   recompressible  decompresses and recompresses a file with go's gzip. Exits with 0 if recompression was transparent
   diff            creates a delta via the rsync algorithm between two files
   help, h         Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```