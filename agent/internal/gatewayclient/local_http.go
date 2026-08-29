package gatewayclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type LocalResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// ExecuteLocal forwards one fully buffered HTTP request to the configured local
// destination. Body streaming and size limits are introduced in later epics.
func ExecuteLocal(ctx context.Context, localAddress string, request Request) (LocalResponse, error) {
	localAddress, err := normalizeLocalAddress(localAddress)
	if err != nil {
		return LocalResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, localAddress+request.Path, bytes.NewReader(request.Body))
	if err != nil {
		return LocalResponse{}, fmt.Errorf("create local request: %w", err)
	}
	httpRequest.Header = withoutHopByHopHeaders(request.Headers)
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return LocalResponse{}, fmt.Errorf("send local request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return LocalResponse{}, fmt.Errorf("read local response: %w", err)
	}
	return LocalResponse{StatusCode: response.StatusCode, Headers: withoutHopByHopHeaders(response.Header), Body: body}, nil
}

func normalizeLocalAddress(localAddress string) (string, error) {
	localAddress = strings.TrimRight(strings.TrimSpace(localAddress), "/")
	if localAddress == "" {
		return "", fmt.Errorf("local address is required")
	}
	if port, err := strconv.ParseUint(localAddress, 10, 16); err == nil && port > 0 {
		localAddress = "127.0.0.1:" + localAddress
	}
	if !strings.HasPrefix(localAddress, "http://") && !strings.HasPrefix(localAddress, "https://") {
		localAddress = "http://" + localAddress
	}
	return localAddress, nil
}

// StreamLocal forwards a request and passes local response metadata and body
// chunks to callbacks without buffering the complete response in the agent.
func StreamLocal(ctx context.Context, localAddress string, request Request, start func(LocalResponse) error, chunk func([]byte) error) error {
	localAddress, err := normalizeLocalAddress(localAddress)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, localAddress+request.Path, bytes.NewReader(request.Body))
	if err != nil {
		return fmt.Errorf("create local request: %w", err)
	}
	httpRequest.Header = withoutHopByHopHeaders(request.Headers)
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("send local request: %w", err)
	}
	defer response.Body.Close()
	if err := start(LocalResponse{StatusCode: response.StatusCode, Headers: withoutHopByHopHeaders(response.Header)}); err != nil {
		return err
	}
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if err := chunk(buffer[:count]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read local response: %w", readErr)
		}
	}
}

func withoutHopByHopHeaders(headers http.Header) http.Header {
	filtered := headers.Clone()
	for _, value := range filtered.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			filtered.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		filtered.Del(name)
	}
	return filtered
}
