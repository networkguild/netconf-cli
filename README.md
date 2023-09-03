# netconf-go

![Release](https://github.devcloud.elisa.fi/NetOps/netconf-go/workflows/Release/badge.svg)
![Master](https://github.devcloud.elisa.fi/NetOps/netconf-go/workflows/Master/badge.svg)

netconf-go is cli tool to execute netconf operations to network devices.

## Installation

- Download binary from latest release
- Copy binary to /usr/local/bin/netconf
- Enjoy

## Usage:

Use provided help pages for more information about subcommands
```
❯ netconf --help
Cli tool for running netconf operations on network devices.

Supported netconf operations are get-config, get, edit-config, copy-config, notifications and custom dispatch.

All commands support parallel run with multiple devices, --inventory, -i file is needed with multiple hosts.

Global flags can be configured via environment variables (prefix NETCONF) or via command-line flags

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
      --host string        IP or hostname of device to connect
  -i, --inventory string   Inventory file containing IP's
      --logfile string     Enables logging to specific file, disables stdout logging
  -p, --password string    SSH password or env NETCONF_PASSWORD (default "admin")
  -P, --port int           Netconf port or env NETCONF_PORT (default 830)
  -u, --username string    SSH username or env NETCONF_USERNAME (default "admin")

Use "netconf [command] --help" for more information about a command.
```
### Inventory file
Flag: `--inventory, -i`

Optional file suffix is used with get, get-config and notification commands. 

See [example](examples/hosts.ini)

### Filters file
Flag: `--filters, -f`

Used with `get`, `get-config` commands. Not available with others.

Currently, filter/subtree per line is supported. There can be any amount of lines (reply will be much bigger).

See [example](examples/filters.xml)

### Commands
All commands below assumes that you have `NETCONF_PASSWORD` and `NETCONF_USERNAME` environment variables set. Or else using defaults.

`Use "netconf [command] --help" for more information about a command` is best resource for examples.

#### Run get-config:
```
Usage:
  netconf get-config [flags]

Flags:
      --save            save config to file, file name is the ip + get
  -s, --source string   datastore, running|candidate|startup (default "running")
```

#### Run get:
```
Usage:
  netconf get [flags]

Flags:
  -d, --defaults string   with-defaults option, report-all|report-all-tagged|trim|explicit
  -f, --filters string    stdin or file containing filters
      --save              save config to file, file name is the filter name + day
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
  -h, --help                help for copy-config
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
      --get             get available notification streams
      --save            save notifications to file, default name is used, if no suffix provided
  -s, --stream string   stream to subscribe (default "NETCONF")
```
