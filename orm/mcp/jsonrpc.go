package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/recoweft/goquent/orm/internal/planversion"
	"github.com/recoweft/goquent/orm/operation"
)

const maxMessageBytes = 1 << 20

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc,omitempty"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HandleJSONRPC handles one JSON-RPC request payload.
func (s *Server) HandleJSONRPC(ctx context.Context, payload []byte) ([]byte, bool) {
	var req rpcRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return marshalRPC(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}}), true
	}
	if req.ID == nil {
		_, _ = s.dispatch(ctx, req)
		return nil, false
	}
	result, err := s.dispatch(ctx, req)
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		resp.Error = &rpcError{Code: -32603, Message: err.Error()}
	} else {
		resp.Result = result
	}
	return marshalRPC(resp), true
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"resources": map[string]any{},
				"tools":     map[string]any{},
				"prompts":   map[string]any{},
			},
			"serverInfo": map[string]any{"name": ServerName, "version": ServerVersion},
		}, nil
	case "resources/list":
		return map[string]any{"resources": s.Resources()}, nil
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			return nil, err
		}
		text, mimeType, err := s.ReadResource(params.URI)
		if err != nil {
			return nil, err
		}
		return map[string]any{"contents": []map[string]any{{"uri": params.URI, "mimeType": mimeType, "text": text}}}, nil
	case "tools/list":
		return map[string]any{"tools": s.Tools()}, nil
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
			Args      map[string]any `json:"args"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			return nil, err
		}
		args := params.Arguments
		if args == nil {
			args = params.Args
		}
		result, err := s.CallTool(ctx, params.Name, args)
		if err != nil {
			return ToolResult{IsError: true, Content: []Content{{Type: "text", Text: err.Error()}}}, nil
		}
		return result, nil
	case "prompts/list":
		return map[string]any{"prompts": s.Prompts()}, nil
	case "prompts/get":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := decodeParams(req.Params, &params); err != nil {
			return nil, err
		}
		messages, err := s.GetPrompt(params.Name, params.Arguments)
		if err != nil {
			return nil, err
		}
		return map[string]any{"messages": messages}, nil
	case "notifications/initialized", "ping":
		return map[string]any{}, nil
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

// Serve runs a minimal MCP stdio JSON-RPC server.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		payload, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		resp, ok := s.HandleJSONRPC(ctx, payload)
		if !ok {
			continue
		}
		if err := writeFramedMessage(w, resp); err != nil {
			return err
		}
	}
}

func decodeParams(params json.RawMessage, out any) error {
	if len(params) == 0 {
		params = []byte(`{}`)
	}
	// Validate nested version declarations before a generic map loses duplicates.
	var envelope struct {
		Arguments map[string]json.RawMessage `json:"arguments"`
		Args      map[string]json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return err
	}
	for _, args := range []map[string]json.RawMessage{envelope.Arguments, envelope.Args} {
		for _, key := range []string{"spec", "operation_spec"} {
			if b, ok := args[key]; ok && len(b) > 0 && b[0] != '"' {
				var spec operation.OperationSpec
				if err := json.Unmarshal(b, &spec); err != nil {
					return err
				}
			}
		}
	}
	return planversion.Decode(params, out)
}

func marshalRPC(resp rpcResponse) []byte {
	b, _ := json.Marshal(resp)
	return b
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	var line string
	for {
		next, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(next, "\r\n")
		if strings.TrimSpace(line) != "" {
			break
		}
	}
	if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
		return []byte(line), nil
	}
	_, lengthText, ok := strings.Cut(line, ":")
	if !ok {
		return nil, fmt.Errorf("invalid Content-Length header")
	}
	length, err := strconv.Atoi(strings.TrimSpace(lengthText))
	if err != nil {
		return nil, err
	}
	if length < 0 || length > maxMessageBytes {
		return nil, fmt.Errorf("invalid Content-Length: %d", length)
	}
	for {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(header) == "" {
			break
		}
	}
	payload := make([]byte, length)
	_, err = io.ReadFull(r, payload)
	return payload, err
}

func writeFramedMessage(w io.Writer, payload []byte) error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(payload))
	b.Write(payload)
	_, err := w.Write(b.Bytes())
	return err
}
