To write this handler I modified the slack handler at https://github.com/sensu/sensu-slack-handler

# Sensu Go Slack Handler

The Sensu Go Alerta handler is a [Sensu Event Handler][1] that sends event data to
an Alerta endpoint.

## Installation

Create an executable script from this source.

From the local path of the alerta-handler repository:
```
go build -o /usr/local/bin/sensu-alerta-handler main.go
```

## Configuration

Example Sensu Go handler definition:


alerta-handler.json

```json
{
	"type": "Handler",
	"api_version": "core/v2",
	"metadata": {
		"namespace": "default",
		"name": "alerta"
	},
	"spec": {
		"type": "pipe",
		"command": "sensu-alerta-handler",
		"env_vars": [
			"ALERTA_ENDPOINT=http://192.168.13.1:8080/alert",
			"ALERTA_ENVIRONMENT=Development",
			"KEY=yourkey",
			"timeout=10"
		],
		"timeout": 30
	}
}
```

`sensuctl create -f alerta-handler.json`

Example Sensu Go check definition:

```json
{
    "api_version": "core/v2",
    "type": "CheckConfig",
    "metadata": {
        "namespace": "default",
        "name": "dummy-app-healthz"
    },
    "spec": {
        "command": "check-http -u http://localhost:8080/healthz",
        "subscriptions":[
            "dummy"
        ],
        "publish": true,
        "interval": 10,
        "handlers": [
            "alerta"
        ]
    }
}
```

## Usage examples

Help:

```
The Sensu Go Alerta handler for notifying a channel

Usage:
  sensu-alerta-handler [flags]

Flags:
  -e, --endpoint string      The http endpoint of alerta
  -E, --environment string   Alerta environment (Development, Production
  -h, --help                 help for sensu-alerta-handler
  -k, --key string           Alerta http auth key
  -t, --timeout int          The amount of seconds to wait before terminating the handler (default 10)
```

[1]: https://docs.sensu.io/sensu-go/5.0/reference/handlers/#how-do-sensu-handlers-work
