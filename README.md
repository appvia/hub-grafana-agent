## **Hub Grafana Agent**

The project teams up with the [appvia-hub](https://github.com/appvia/appvia-hub) providing the ability to provision Grafana dashboards.

```
NAME:
   hub-grafana-agent A backend agent used to provision dashboards in Grafana.

USAGE:
    [global options] command [command options] [arguments...]

VERSION:
   v0.0.1

AUTHOR:
   Daniel Whatmuff <daniel.whatmuff@appvia.io>

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --listen INTERFACE  the interface to bind the service to INTERFACE (default: "127.0.0.1") [$LISTEN]
   --http-port PORT    network interface the service should listen on PORT (default: "10080") [$HTTP_PORT]
   --https-port PORT   network interface the service should listen on PORT (default: "10443") [$HTTPS_PORT]
   --auth-token TOKEN  authentication token used to verifier the caller TOKEN [$AUTH_TOKEN]
   --tls-cert PATH     the path to the file containing the certificate pem PATH [$TLS_CERT]
   --tls-key PATH      the path to the file containing the private key pem PATH [$TLS_KEY]
   --help, -h          show help
   --version, -v       print the version
```

#### **Grafana Authentication**

In order to speak to your Grafana API you will need to provision an [API key](https://grafana.com/docs/tutorials/api_org_token_howto/) with the `Admin` Grafana role.
