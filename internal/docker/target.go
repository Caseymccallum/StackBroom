package docker

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Where the active endpoint came from. Docker's precedence is: DOCKER_CONTEXT
// selects a context, and a named context's endpoint takes precedence over
// DOCKER_HOST. The special "default" context derives its endpoint from
// DOCKER_HOST, so inspecting it reports that value (verified against Docker
// 29.4.0 on 27 Oct 2026). StackBroom always classifies the endpoint Docker
// itself reports, so the refusal decision follows the CLI's effective target.
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
	Local bool
	// Valid is true only for a well-formed Docker daemon address. An invalid
	// endpoint is never contacted, even with --allow-remote-docker, because
	// "unknown" must not become "connect anyway".
	Valid  bool
	Reason string
}

// dockerEndpointKey is the only context endpoint type that addresses a Docker
// daemon. Other keys (for example a future transport) must never be selected.
const dockerEndpointKey = "docker"

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
	// Valid is true only when the endpoint is a well-formed Docker address.
	Valid  bool
	Reason string
}

// Approved reports whether StackBroom may connect to this endpoint. An invalid
// endpoint is never approved, even when remote access was explicitly allowed.
func (t Target) Approved(allowRemote bool) bool {
	return t.Valid && (t.Local || allowRemote)
}

// ClassifyEndpoint decides whether an endpoint is a local IPC channel and
// whether it is a well-formed Docker daemon address.
//
// Only canonical local npipe, unix and fd endpoints are considered local. TCP
// and SSH are network transports and require explicit approval, including
// loopback addresses. Malformed or unsupported endpoints are marked invalid and
// are never contacted, even with --allow-remote-docker.
func ClassifyEndpoint(raw string) EndpointClass {
	e := strings.TrimSpace(raw)
	if e == "" {
		return invalidEndpoint(transportUnknown, "no Docker endpoint is configured")
	}
	colon := strings.IndexByte(e, ':')
	if colon <= 0 {
		return invalidEndpoint(transportUnknown, "the Docker endpoint has no recognised transport scheme")
	}
	scheme := strings.ToLower(strings.TrimSpace(e[:colon]))
	rest := strings.TrimSpace(e[colon+1:])

	// A drive letter (C:\...) is a path, not a transport scheme.
	if len(scheme) == 1 {
		return invalidEndpoint(transportUnknown, "the Docker endpoint has no recognised transport scheme")
	}

	switch scheme {
	case transportNamedPipe:
		return classifyNamedPipe(rest)
	case transportUnix:
		return classifyUnixSocket(rest)
	case transportFD:
		return classifyFileDescriptor(strings.TrimLeft(rest, "/"))
	case transportTCP, transportSSH:
		return classifyNetworkEndpoint(e, scheme)
	case transportHTTP, transportHTTPS:
		return invalidEndpoint(scheme,
			"the "+scheme+" transport is not supported by the Docker CLI; use tcp, ssh, npipe, unix or fd")
	default:
		return invalidEndpoint(transportUnknown, "the Docker endpoint uses an unsupported transport scheme")
	}
}

func invalidEndpoint(transport, reason string) EndpointClass {
	return EndpointClass{Transport: transport, Valid: false, Reason: reason}
}

// classifyNamedPipe accepts only the canonical local form
// npipe:////./pipe/<name>. Host-qualified forms (a UNC path such as
// npipe:////server/pipe/<name>) address another machine and are valid but never
// local. Malformed or ambiguous forms are invalid.
func classifyNamedPipe(rest string) EndpointClass {
	if rest == "" || rest == "//" {
		return invalidEndpoint(transportNamedPipe, "the named pipe endpoint has no path")
	}
	if strings.ContainsAny(rest, "?#@\\%") {
		return invalidEndpoint(transportNamedPipe, "the named pipe endpoint contains unsupported characters")
	}
	if !strings.HasPrefix(rest, "////") {
		return invalidEndpoint(transportNamedPipe,
			"the named pipe endpoint is not in the form npipe:////./pipe/<name>")
	}

	path := rest[4:]
	const localPrefix = "./pipe/"
	if strings.HasPrefix(strings.ToLower(path), localPrefix) {
		name := path[len(localPrefix):]
		if !validPipeName(name) {
			return invalidEndpoint(transportNamedPipe, "the named pipe name is empty or malformed")
		}
		return EndpointClass{
			Transport: transportNamedPipe,
			Local:     true,
			Valid:     true,
			Reason:    "local named pipe",
		}
	}

	// Anything else is not a local Docker Desktop pipe. Host-qualified UNC forms
	// address another machine over SMB; StackBroom does not support remote named
	// pipes, so they are invalid and are never contacted, even with
	// --allow-remote-docker. Remote daemons must use tcp or ssh.
	return invalidEndpoint(transportNamedPipe,
		"host-qualified or non-canonical named pipes are not supported; use tcp or ssh for a remote daemon")
}

func classifyUnixSocket(rest string) EndpointClass {
	// rest still includes the URL authority slashes: unix:///path -> "///path".
	if strings.HasPrefix(rest, "//") {
		rest = rest[2:]
	}
	if rest == "" {
		return invalidEndpoint(transportUnix, "the Unix socket endpoint has no path")
	}
	if strings.ContainsAny(rest, "?#") {
		return invalidEndpoint(transportUnix, "the Unix socket endpoint contains unsupported characters")
	}
	if !strings.HasPrefix(rest, "/") || rest == "/" {
		return invalidEndpoint(transportUnix, "the Unix socket endpoint path is not absolute")
	}
	return EndpointClass{
		Transport: transportUnix,
		Local:     true,
		Valid:     true,
		Reason:    "local Unix socket",
	}
}

func classifyFileDescriptor(rest string) EndpointClass {
	if rest == "" {
		return invalidEndpoint(transportFD, "the file descriptor endpoint has no descriptor")
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return invalidEndpoint(transportFD, "the file descriptor endpoint is not numeric")
		}
	}
	return EndpointClass{
		Transport: transportFD,
		Local:     true,
		Valid:     true,
		Reason:    "local file descriptor",
	}
}

func classifyNetworkEndpoint(endpoint, scheme string) EndpointClass {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return invalidEndpoint(scheme, "the Docker endpoint could not be parsed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return invalidEndpoint(scheme, "the Docker endpoint has an unsupported query or fragment")
	}
	// SSH legitimately carries a user; a TCP address does not.
	if scheme == transportTCP && parsed.User != nil {
		return invalidEndpoint(scheme, "the Docker endpoint contains credentials in the address")
	}
	host := parsed.Hostname()
	if host == "" {
		return invalidEndpoint(scheme, "the Docker endpoint has no host")
	}

	reason := fmt.Sprintf("%s endpoint %s is a network destination", scheme, displayEndpoint(endpoint))
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		reason += " on the loopback interface, which may still forward to a remote daemon"
	} else if strings.EqualFold(host, "localhost") {
		reason += " on localhost, which may still forward to a remote daemon"
	}
	return EndpointClass{
		Transport: scheme,
		Host:      host,
		Local:     false,
		Valid:     true,
		Reason:    reason,
	}
}

func validPipeName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if !isPipeChar(r) {
			return false
		}
	}
	return true
}

func isPipeChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '.', r == '_', r == '-':
		return true
	default:
		return false
	}
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
