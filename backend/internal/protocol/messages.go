package protocol

const Version = 1

const (
	ClientHello       = "client_hello"
	ServerHello       = "server_hello"
	Ping              = "ping"
	Pong              = "pong"
	OpenTunnel        = "open_tunnel"
	TunnelOpened      = "tunnel_opened"
	CloseTunnel       = "close_tunnel"
	HTTPRequestStart  = "http_request_start"
	HTTPRequestEnd    = "http_request_end"
	HTTPResponseStart = "http_response_start"
	HTTPResponseEnd   = "http_response_end"
	CancelRequest     = "cancel_request"
	Error             = "error"
)

type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type CancelRequestPayload struct {
	RequestID string `json:"requestId"`
}
type ClientHelloPayload struct {
	ProtocolVersion int    `json:"protocolVersion"`
	AgentVersion    string `json:"agentVersion"`
	InstanceID      string `json:"instanceId"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}
type ServerHelloPayload struct {
	ProtocolVersion          int    `json:"protocolVersion"`
	SessionID                string `json:"sessionId"`
	HeartbeatIntervalSeconds int    `json:"heartbeatIntervalSeconds"`
}

type OpenTunnelPayload struct {
	RequestID    string `json:"requestId"`
	LocalAddress string `json:"localAddress"`
}

type TunnelOpenedPayload struct {
	RequestID string `json:"requestId"`
	TunnelID  string `json:"tunnelId"`
	Subdomain string `json:"subdomain"`
	PublicURL string `json:"publicUrl"`
}

type CloseTunnelPayload struct {
	RequestID string `json:"requestId"`
	TunnelID  string `json:"tunnelId"`
}

type HTTPRequestPayload struct {
	RequestID     string              `json:"requestId"`
	Method        string              `json:"method"`
	Path          string              `json:"path"`
	Headers       map[string][]string `json:"headers"`
	ContentLength int64               `json:"contentLength"`
}

type HTTPRequestEndPayload struct {
	RequestID string `json:"requestId"`
}

type HTTPResponsePayload struct {
	RequestID     string              `json:"requestId"`
	StatusCode    int                 `json:"statusCode"`
	Headers       map[string][]string `json:"headers"`
	ContentLength int64               `json:"contentLength"`
	Error         string              `json:"error,omitempty"`
}

type HTTPResponseEndPayload struct {
	RequestID string `json:"requestId"`
}
