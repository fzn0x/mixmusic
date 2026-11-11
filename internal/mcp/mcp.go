// Package mcp lets the AI assistants of this computer control MixMusic
// over the Model Context Protocol (#477). An assistant starts a command,
// MixMusic with --mcp (Relay), which passes the protocol's messages, one
// JSON-RPC message per line as the stdio transport has them, between the
// assistant and the server of the MixMusic that runs (Server). The server
// listens on 127.0.0.1 alone, and lets in a connection only with the key of
// the file it keeps in the data folder while it runs, which the user alone
// can read.
//
// It speaks the protocol's revision 2026-07-28, whose requests each carry
// their version, and the revisions before it, from 2024-11-05, whose
// sessions start with initialize. It offers tools, and nothing else: no
// resources, no prompts, and no requests of its own to the assistant.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// Latest is the revision of the protocol whose requests each carry their
// version, in their _meta.
const Latest = "2026-07-28"

// legacy are the revisions whose sessions start with initialize, newest
// first.
var legacy = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// versions are the revisions the server speaks, newest first.
var versions = append([]string{Latest}, legacy...)

// Tool is what an assistant may call.
type Tool struct {
	// Name names it in calls: letters, digits and underscores.
	Name string
	// Title is its name for people, and Description tells the assistant
	// what it does and when to call it.
	Title       string
	Description string
	// Input is the JSON Schema of its arguments, an object.
	Input json.RawMessage
	// ReadOnly is set for a tool that changes nothing, Idempotent for one
	// that does no more when called again with the same arguments, and
	// OpenWorld for one that asks services on the internet.
	ReadOnly, Idempotent, OpenWorld bool
	// Call runs it. An error is the tool's, which the assistant reads to
	// try again otherwise: an argument out of range, nothing to play.
	Call func(ctx context.Context, args json.RawMessage) (Result, error)
}

// Result is what a tool tells the assistant: Text, and Data, which goes
// as the result's structured content too, when it isn't nil.
type Result struct {
	Text string
	Data any
}

// Args reads the arguments of a call into a T, which names each argument
// in its fields' json tags: an argument it doesn't name, or one of the
// wrong type, is an error the assistant can read.
func Args[T any](args json.RawMessage) (T, error) {
	var v T
	args = bytes.TrimSpace(args)
	if len(args) == 0 || bytes.Equal(args, []byte("null")) {
		return v, nil
	}
	d := json.NewDecoder(bytes.NewReader(args))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return v, fmt.Errorf("the argument %s isn't %s", typeErr.Field, jsonKind(typeErr.Type))
		}
		return v, fmt.Errorf("the arguments don't read: %v", err)
	}
	return v, nil
}

// jsonKind is what a value of t is in JSON, for the assistant.
func jsonKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "a list"
	case reflect.Pointer:
		return jsonKind(t.Elem())
	}
	return "an object"
}

// definition is a tool as tools/list tells it.
func (t Tool) definition() map[string]any {
	input := t.Input
	if len(input) == 0 {
		input = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	}
	d := map[string]any{"name": t.Name, "description": t.Description, "inputSchema": input}
	if t.Title != "" {
		d["title"] = t.Title
	}
	notes := map[string]any{"readOnlyHint": t.ReadOnly, "openWorldHint": t.OpenWorld}
	if t.Title != "" {
		notes["title"] = t.Title
	}
	if !t.ReadOnly {
		// Nothing the tools do takes back what the user had: the queue
		// changes, which the user changes again.
		notes["destructiveHint"] = false
		notes["idempotentHint"] = t.Idempotent
	}
	d["annotations"] = notes
	return d
}
