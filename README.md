<p align="center">
    <img alt="gobin logo" src="assets/gobin-logo.png" height="150" />
    <h3 align="center">gobin</h3>
    <p align="center">CLI to manage Go binaries</p>
</p>

---

**gobin** is a CLI tool that leverages the Go toolchain to list, inspect, upgrade, uninstall and diagnose Go binaries installed in the system.

[![ci](https://img.shields.io/github/actions/workflow/status/brunoribeiro127/gobin/ci.yml?&branch=main)](https://github.com/brunoribeiro127/gobin/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/brunoribeiro127/gobin.svg)](https://github.com/brunoribeiro127/gobin/releases/latest)
[![license](https://img.shields.io/badge/license-MIT%20or%20Apache--2.0-blue.svg)](#license)
[![go version](https://img.shields.io/github/go-mod/go-version/brunoribeiro127/gobin)](./go.mod)
[![go reference](https://pkg.go.dev/badge/github.com/brunoribeiro127/gobin.svg)](https://pkg.go.dev/github.com/brunoribeiro127/gobin)
[![codecov](https://codecov.io/github/brunoribeiro127/gobin/graph/badge.svg?token=KPQGGWGGCC)](https://codecov.io/github/brunoribeiro127/gobin)
[![go report card](https://goreportcard.com/badge/github.com/brunoribeiro127/gobin)](https://goreportcard.com/report/github.com/brunoribeiro127/gobin)

## Features

- Inspect binary information
- List and show outdated binaries
- Upgrade individual or all outdated binaries
- Install, uninstall and pin binaries
- Diagnose and troubleshoot issues with installed binaries
- Migrate binaries to be managed internally

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

| Command                | Description                                 | Flags                                                                                             |
|------------------------|---------------------------------------------|---------------------------------------------------------------------------------------------------|
| `completion [shell]`   | Generate shell completion scripts           |                                                                                                   |
| `doctor`               | Diagnose issues for binaries                |                                                                                                   |
| `info [binary]`        | Show info about a binary                    |                                                                                                   |
| `install [packages]`   | Install and pin packages                    | `-k`, `--kind` – pin kind: [latest (default), major, minor]<br>`-r`, `--rebuild` – force package rebuild |
| `list`                 | List installed binaries                     | `-m`, `--managed` – list all managed binaries                                                                                                   |
| `migrate [binaries]`   | Migrate binaries to be managed internally   | `-a`, `--all` – migrate all binaries in the Go binary path                                        |
| `outdated`             | List outdated binaries                      | `-m`, `--major` – include major version updates                                                   |
| `pin [binaries]`       | Pin binaries to the Go binary path          | `-k`, `--kind` – pin kind: [latest (default), major, minor]                                       |
| `repo [binary]`        | Show binary repository                      | `-o`, `--open` – open repository URL in the default browser                                       |
| `uninstall [binaries]` | Uninstall binaries                          |                                                                                                   |
| `upgrade [binaries]`   | Upgrade specific binaries or all with --all | `-a`, `--all` – upgrade all outdated binaries<br>`-m`, `--major` – allow major version upgrade<br>`-r`, `--rebuild` – force binary rebuild |
| `version`              | Show version info                           | `-s`, `--short` – print short version                                                             |

### Global Flags

These flags can be used with any command:

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Enable verbose output (debug logging) |
| `-p`, `--parallelism` | Number of concurrent operations (default: number of CPU cores) |

## Binary Management

Installation of multiple versions of the same binary is supported. The `GOBIN` environment variable is used to redirect the binary installation to an internal temporary directory. Then, the binary is moved to the internal binary path with the format `<binary>@<version>`. Finally, a symbolic link is created to the Go binary path to ensure the binary is available in the system path.

Binaries are installed internally in the following paths:

- Linux/MacOS: `$HOME/.gobin/bin`
- Windows: `%USERPROFILE%\AppData\Local\gobin\bin`

The Go binary installation path is determined by the following:
- checks if the `GOBIN` environment variable is set
- if not, checks if the `GOPATH` environment variable is set
- if not, use the default path `$HOME/go/bin`

Although not recommended, it is possible to manage multiple binary paths by passing the `GOBIN` or `GOPATH` environment variables to the command. The tool leverages the Go toolchain and injects all Go environment variables to the commands used to manage binaries. The support for private modules is guaranteed by setting the `GOPRIVATE` environment variable.

## License

This project is dual-licensed under [MIT](LICENSE-MIT) or [Apache 2.0](LICENSE-APACHE).
