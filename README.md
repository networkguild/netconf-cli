# netconf-cli

[![Main](https://img.shields.io/github/actions/workflow/status/networkguild/netconf-cli/main.yaml?style=social)](https://github.com/networkguild/netconf-cli/actions/workflows/main.yaml)
[![Releases](https://img.shields.io/github/release/networkguild/netconf-cli/all.svg?style=social)](https://github.com/networkguild/netconf-cli/releases)

netconf-cli is cli tool to execute netconf operations to network devices.

## Installation

- Download binary from latest release
- Copy binary to /usr/local/bin/netconf (or any other location in $PATH)
- Enjoy

#### Note for apple users

If binary doesn't run, or shell kills it instantly, you might need to allow it. From terminal `sudo xattr -d com.apple.quarantine /path/to/binary` 
or from settings under `Privacy & Security`

## Usage:

Use provided help pages for more information about subcommands
```
❯ netconf --help
Cli tool for running netconf operations on network devices.

Supported netconf operations are get-config, get, edit-config, copy-config, notifications and custom dispatch.

All commands support parallel run with multiple devices, --inventory, -i file is needed with multiple hosts.

Global flags can be configured via environment variables (prefix NETCONF) or via command-line flags

If you want trace all incoming and outgoing RPC's, set NETCONF_DEBUG_CAPTURE_DIR environment variable or use --trace flag,
this will save all incoming RPC's to file <currect-time>.in and outgoing RPC's to <currect-time>.out.
RPC's are saved in raw format, including chunked markers.

Usage:
  netconf [command]

Available Commands:
  completion   Generate completion script
  copy-config  Execute copy-config rpc
  dispatch     Execute rpc
  edit-config  Execute edit-config rpc
  get          Execute get rpc
  get-config   Execute get-config rpc
  help         Help about any command
  notification Execute create-subscription rpc

Flags:
      --caller             Enables logging to show caller func
      --debug              Enables debug level logging, logs raw replies
  -h, --help               help for netconf
      --host string        IP or IP's of devices to connect
  -i, --inventory string   Inventory file containing IP's
      --logfile string     Enables logging to specific file, disables stdout logging
  -p, --password string    SSH password or env NETCONF_PASSWORD (default "admin")
  -P, --port int           Netconf port or env NETCONF_PORT (default 830)
      --trace              Enables RPC tracing, saves all incoming and outgoing RPC's to file. Default dir $HOME/.netconf
  -u, --username string    SSH username or env NETCONF_USERNAME (default "admin")

Use "netconf [command] --help" for more information about a command.
```
### Inventory file
Flag: `--inventory, -i`

Optional file suffix is used with get, get-config and notification commands. 

See [example](examples/hosts.ini)

### Filters file
Flag: `--filter, -f`

Used with `get`, `get-config` commands. Not available with others.

Currently, filter/subtree per line is supported. There can be any amount of lines (reply will be much bigger).

See [example](examples/filters.xml)

### Edit-config & dispatch xml
Flag: `--file, -f`

Used with `dispatch`, `edit-config` commands. Not available with others.

Flag value could be file or directory containing multiple files.
Files are executed in alphabetical order.

Example directory structure:

See [dispatch example](examples/dispatch)
See [edit-config example](examples/edit-config)

### Commands
All commands below assumes that you have `NETCONF_PASSWORD` and `NETCONF_USERNAME` environment variables set, or else using defaults.
Global flags are available for all commands, see above or `netconf --help`.

`Use "netconf [command] --help" for more information about a command` is best resource for examples.

#### Run get-config:
```
Usage:
  netconf get-config [flags]

Flags:
  -f, --filter string          filter option, stdin or file containing filters
      --save                   save output to file, default name is used, if no suffix provided
  -s, --source string          running|candidate|startup (default "running")
  -d, --with-defaults string   with-defaults option, report-all|report-all-tagged|trim|explicit
```

#### Run get:
```
Usage:
  netconf get [flags]

Flags:
  -f, --filter string          filter option, stdin or file containing filters
      --save                   save output to file, default name is used, if no suffix provided
  -d, --with-defaults string   with-defaults option, report-all|report-all-tagged|trim|explicit
```

#### Run edit-config:
```
Usage:
  netconf edit-config [flags]

Flags:
  -c, --copy                       run copy-config after rpc's
  -d, --default-operation string   default-operation, none|merge|remove (default "merge")
  -f, --file string                stdin, file or directory containing xml files
  -t, --test-option string         test-option, test-then-set|set|test-only
```

#### Run copy-config:
```
Usage:
  netconf copy-config [flags]

Flags:
  -s, --source string       source configuration datastore
  -S, --source-url string   source configuration url
  -t, --target string       target configuration datastore to save config
  -T, --target-url string   target configuration url to save config
```

#### Run dispatch (run any rpc)
```
Usage:
  netconf dispatch [flags]

Flags:
  -f, --file string   stdin, file or directory containing xml files
  -l, --lock          run with datastore lock
```

#### Run notification (will run until ctrl+c or provided end time)
```
Usage:
  netconf notification [flags]

Flags:
  -d, --duration duration   duration for subscription, eg. 2h30m45s
      --get                 get available notification streams
      --save                save notifications to file, default name is used, if no suffix provided
  -s, --stream string       stream to subscribe (default "NETCONF")
```
