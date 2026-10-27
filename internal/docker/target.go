package docker

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Where the active endpoint came from. These values mirror Docker's documented
// precedence: DOCKER_CONTEXT overrides DOCKER_HOST, which overrides the context
// selected with `docker context use`.
const (
	SourceContextEnv = "DOCKER_CONTEXT"
	SourceHostEnv    = "DOCKER_HOST"
	SourceActiveCtx  = "active-context"
	SourceUnknown    = "unknown"
)

// EndpointClass is the result of classifying a Docker endpoint string.
type EndpointClass struct {
	Transport string
	Host      string
	// Local is true only for IPC channels that cannot address another machine.
	Local  bool
	Reason string
}

// Local IPC transports. A TCP endpoint is never treated as local, even on
// loopback, because a local port can forward to a remote daemon (an SSH tunnel,
// for example).
const (
	transportNamedPipe = "npipe"
	transportUnix      = "unix"
	transportFD        = "fd"
	transportTCP       = "tcp"
	transportSSH       = "ssh"
	transportHTTP      = "http"
	transportHTTPS     = "https"
	transportUnknown   = "unknown"
)

// Target is the resolved Docker endpoint StackBroom would use.
type Target struct {
	// Endpoint is the raw configured value (not for display; it may contain
	// credentials).
	Endpoint string
	// Display is the endpoint with userinfo redacted.
	Display string
	// Context is the active context name, when it could be determined.
	Context string
	Source  string
	Local   bool
	Reason  string
}

// Approved reports whether StackBroom may connect to this endpoint.
func (t Target) Approved(allowRemote bool) bool { return t.Local || allowRemote }

// ClassifyEndpoint decides whether an endpoint is a local IPC channel.
//
// Only npipe, unix and fd are considered local. tcp, http, https and ssh are
// network transports and require explicit approval, including loopback
// addresses.
func ClassifyEndpoint(raw string) EndpointClass {
	e := strings.TrimSpace(raw)
	if e == "" {
		return EndpointClass{Transport: transportUnknown, Reason: "no Docker endpoint is configured"}
	}
	lower := strings.ToLower(e)
	colon := strings.IndexByte(lower, ':')
	if colon <= 0 {
		return EndpointClass{Transport: transportUnknown, Reason: "the Docker endpoint has no recognised transport scheme"}
	}
	scheme := lower[:colon]
	rest := strings.TrimSpace(e[colon+1:])

	// A drive letter (C:\...) is a path, not a transport scheme.
	if len(scheme) == 1 {
		return EndpointClass{Transport: transportUnknown, Reason: "the Docker endpoint has no recognised transport scheme"}
	}

	switch scheme {
	case "npipe":
		if rest == "" || rest == "//" {
			return EndpointClass{Transport: transportNamedPipe, Reason: "the named pipe endpoint has no path"}
		}
		return EndpointClass{Transport: transportNamedPipe, Local: true, Reason: "local named pipe"}
	case "unix":
		if rest == "" || rest == "///" {
			return EndpointClass{Transport: transportUnix, Reason: "the Unix socket endpoint has no path"}
		}
		return EndpointClass{Transport: transportUnix, Local: true, Reason: "local Unix socket"}
	case "fd":
		return EndpointClass{Transport: transportFD, Local: true, Reason: "local file descriptor"}
	}

	parsed, err := url.Parse(e)
	if err != nil {
		return EndpointClass{Transport: transportUnknown, Reason: "the Docker endpoint could not be parsed"}
	}
	host := parsed.Hostname()
	transport := scheme
	switch scheme {
	case "tcp", "http", "https", "ssh":
		// Known network transports.
	default:
		transport = transportUnknown
	}
	if host == "" {
		return EndpointClass{Transport: transport, Reason: "the Docker endpoint has no host"}
	}

	reason := fmt.Sprintf("%s endpoint %s is a network destination", transport, displayEndpoint(e))
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		reason += " on the loopback interface, which may still forward to a remote daemon"
	} else if strings.EqualFold(host, "localhost") {
		reason += " on localhost, which may still forward to a remote daemon"
	}
	return EndpointClass{Transport: transport, Host: host, Reason: reason}
}

// displayEndpoint removes credentials from an endpoint for display.
func displayEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "(unparseable endpoint)"
	}
	if parsed.User != nil {
		parsed.User = url.User(parsed.User.Username())
	}
	return parsed.Redacted()
}

func orUnknownEndpoint(endpoint string) string {
	if strings.TrimSpace(endpoint) == "" {
		return "(unresolved endpoint)"
	}
	return endpoint
}
