<p align="center">
    <img alt="gobin logo" src="assets/gobin-logo.png" height="150" />
    <h3 align="center">gobin</h3>
    <p align="center">CLI to manage Go binaries</p>
</p>

---

**gobin** is a CLI tool that leverages the Go toolchain to list, inspect, upgrade, uninstall and diagnose Go binaries installed in the system.

[![CI](https://img.shields.io/github/actions/workflow/status/brunoribeiro127/gobin/ci.yml?&branch=main)](https://github.com/brunoribeiro127/gobin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/release/brunoribeiro127/gobin.svg)](https://github.com/brunoribeiro127/gobin/releases/latest)
[![License](https://img.shields.io/badge/license-MIT%20or%20Apache--2.0-blue.svg)](#license)
[![Go Reference](https://pkg.go.dev/badge/github.com/brunoribeiro127/gobin.svg)](https://pkg.go.dev/github.com/brunoribeiro127/gobin)
[![Go Report Card](https://goreportcard.com/badge/github.com/brunoribeiro127/gobin)](https://goreportcard.com/report/github.com/brunoribeiro127/gobin)

## Features

- Inspect binary information
- List and show outdated binaries
- Upgrade individual or all outdated binaries
- Uninstall binaries
- Diagnose and troubleshoot issues with installed binaries

## Installation

### Via Go Module System

```sh
go install github.com/brunoribeiro127/gobin@latest
```

### Via Prebuilt Binaries

**Linux/MacOS:**
```sh
curl -L -o gobin_<version>_<os>_<arch>.tar.gz https://github.com/brunoribeiro127/gobin/releases/download/v<version>/gobin_<version>_<os>_<arch>.tar.gz
tar -xzf gobin_<version>_<os>_<arch>.tar.gz
mv gobin $HOME/go/bin  # adjust to your Go binaries path
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest https://github.com/brunoribeiro127/gobin/releases/download/v<version>/gobin_<version>_windows_<arch>.tar.gz -OutFile gobin.tar.gz
tar -xzf gobin.tar.gz
Move-Item .\gobin.exe "$env:USERPROFILE\go\bin"  # adjust to your Go binaries path
```

> **Note:** All releases include checksums and cryptographic signatures for verification. Use [cosign](https://docs.sigstore.dev/cosign/installation/) to verify signatures if desired.

### Via Source

Requirements:
- Go >= v1.24
- [Taskfile](https://taskfile.dev/installation/) >= v3

```sh
git clone https://github.com/brunoribeiro127/gobin.git
cd gobin
task install
```

## Usage

| Command              | Description                                 | Flags                                                                                             |
|----------------------|---------------------------------------------|---------------------------------------------------------------------------------------------------|
| `completion [shell]` | Generate shell completion scripts           |                                                                                                   |
| `doctor`             | Diagnose issues for binaries                |                                                                                                   |
| `info [binary]`      | Show info about a binary                    |                                                                                                   |
| `list`               | List installed binaries                     |                                                                                                   |
| `outdated`           | List outdated binaries                      | `-m`, `--major` – include major version updates                                                   |
| `repo [binary]`      | Show binary repository                      | `-o`, `--open` – open repository URL in the default browser                                       |
| `uninstall [binary]` | Uninstall a binary                          |                                                                                                   |
| `upgrade [binaries]` | Upgrade specific binaries or all with --all | `-m`, `--major` – allow major version upgrade<br>`-a`, `--all` – upgrade all outdated binaries<br>`-r`, `--rebuild` – force binary rebuild |
| `version`            | Show version info                           | `-s`, `--short` – print short version                                                             |

### Global Flags

These flags can be used with any command:

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Enable verbose output (debug logging) |
| `-p`, `--parallelism` | Number of concurrent operations (default: number of CPU cores) |

## Installation Path

The installation path for binaries is determined by the following flow:
- checks if the `GOBIN` environment variable is set
- if not, checks if the `GOPATH` environment variable is set
- if not, use the default path `$HOME/go/bin`

Although not recommended, it is possible to manage multiple binary paths by passing the `GOBIN` or `GOPATH` environment variables to the command. The tool leverages the Go toolchain and injects all Go environment variables to the commands used to manage binaries. The support for private modules is guaranteed by setting the `GOPRIVATE` environment variable.

## License

This project is dual-licensed under [MIT](LICENSE-MIT) or [Apache 2.0](LICENSE-APACHE).
