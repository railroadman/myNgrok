package gatewayclient

const openTunnelType = "open_tunnel"
const tunnelOpenedType = "tunnel_opened"
const closeTunnelType = "close_tunnel"
const cancelRequestType = "cancel_request"
const httpRequestStartType = "http_request_start"
const httpRequestEndType = "http_request_end"
const httpResponseStartType = "http_response_start"
const httpResponseEndType = "http_response_end"

type openTunnelPayload struct {
	RequestID    string `json:"requestId"`
	LocalAddress string `json:"localAddress"`
}

type cancelRequestPayload struct {
	RequestID string `json:"requestId"`
}

type tunnelOpenedPayload struct {
	RequestID string `json:"requestId"`
	TunnelID  string `json:"tunnelId"`
	Subdomain string `json:"subdomain"`
	PublicURL string `json:"publicUrl"`
}

type closeTunnelPayload struct {
	RequestID string `json:"requestId"`
	TunnelID  string `json:"tunnelId"`
}

type httpRequestPayload struct {
	RequestID     string              `json:"requestId"`
	Method        string              `json:"method"`
	Path          string              `json:"path"`
	Headers       map[string][]string `json:"headers"`
	ContentLength int64               `json:"contentLength"`
}

type httpRequestEndPayload struct {
	RequestID string `json:"requestId"`
}

type httpResponsePayload struct {
	RequestID     string              `json:"requestId"`
	StatusCode    int                 `json:"statusCode"`
	Headers       map[string][]string `json:"headers"`
	ContentLength int64               `json:"contentLength"`
	Error         string              `json:"error,omitempty"`
}

type httpResponseEndPayload struct {
	RequestID string `json:"requestId"`
}
