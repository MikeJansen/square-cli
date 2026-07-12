// Package commands contains the runtime helpers shared by all generated
// command files plus the generated command tree itself (the *_gen.go files).
//
// The generated code is intentionally thin: each operation assembles its
// request path, query parameters, and optional JSON body, then delegates to
// Execute, which performs client-side validation, issues the HTTP call, and
// renders the response.
package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/MikeJansen/square-cli/internal/client"
	"github.com/MikeJansen/square-cli/internal/output"
	"github.com/spf13/cobra"
)

// Standard flag names used by operations that accept a request body.
const (
	FlagBody     = "body"
	FlagBodyFile = "body-file"
	FlagDocs     = "docs"
	FlagTyped    = "typed"
)

// Runtime carries the resolved client and output settings through the command
// context.
type Runtime struct {
	Client *client.Client
	Output output.Options
}

type contextKey struct{}

// WithRuntime returns a context carrying the runtime.
func WithRuntime(ctx context.Context, rt *Runtime) context.Context {
	return context.WithValue(ctx, contextKey{}, rt)
}

// runtimeFrom extracts the runtime from a command's context.
func runtimeFrom(cmd *cobra.Command) (*Runtime, error) {
	rt, ok := cmd.Context().Value(contextKey{}).(*Runtime)
	if !ok || rt == nil {
		return nil, errors.New("internal error: runtime not initialized")
	}
	return rt, nil
}

// Op describes a single API operation to Execute.
type Op struct {
	Method        string
	Path          string // fully substituted request path
	Query         url.Values
	HasBody       bool
	RequestProto  func() any // returns a pointer to the typed request, for validation
	ResponseProto func() any // returns a pointer to the typed response, for --typed output
	DocURL        string
}

// Execute reads any request body, validates it against the typed model, issues
// the API call, and renders the response.
func Execute(cmd *cobra.Command, op Op) error {
	if d, _ := cmd.Flags().GetBool(FlagDocs); d {
		fmt.Fprintln(cmd.OutOrStdout(), op.DocURL)
		return nil
	}

	rt, err := runtimeFrom(cmd)
	if err != nil {
		return err
	}

	var bodyBytes []byte
	if op.HasBody {
		bodyBytes, err = readBody(cmd)
		if err != nil {
			return err
		}
		if len(bodyBytes) > 0 && op.RequestProto != nil {
			if err := validateJSON(bodyBytes, op.RequestProto()); err != nil {
				return fmt.Errorf("invalid request body: %w", err)
			}
		}
	}

	resp, err := rt.Client.Do(cmd.Context(), client.Request{
		Method: op.Method,
		Path:   op.Path,
		Query:  op.Query,
		Body:   client.BodyReader(bodyBytes),
	})
	if err != nil {
		// Still render the error body if present, then return the error.
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && len(apiErr.Raw) > 0 {
			_ = rt.Output.WriteBody(apiErr.Raw)
		}
		return err
	}

	// Optional typed round-trip for normalized output.
	if typed, _ := cmd.Flags().GetBool(FlagTyped); typed && op.ResponseProto != nil {
		v := op.ResponseProto()
		if err := json.Unmarshal(resp.Body, v); err == nil {
			return rt.Output.WriteValue(v)
		}
		// Fall through to raw rendering if typed decode fails.
	}

	return rt.Output.WriteBody(resp.Body)
}

// readBody resolves the request body from --body, --body-file, or stdin ("-").
func readBody(cmd *cobra.Command) ([]byte, error) {
	inline, _ := cmd.Flags().GetString(FlagBody)
	file, _ := cmd.Flags().GetString(FlagBodyFile)

	switch {
	case inline != "" && file != "":
		return nil, fmt.Errorf("--%s and --%s are mutually exclusive", FlagBody, FlagBodyFile)
	case inline != "":
		return []byte(inline), nil
	case file == "-":
		return io.ReadAll(cmd.InOrStdin())
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read --%s: %w", FlagBodyFile, err)
		}
		return data, nil
	default:
		return nil, nil
	}
}

// validateJSON unmarshals data into proto rejecting unknown fields.
func validateJSON(data []byte, proto any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(proto); err != nil {
		return err
	}
	return nil
}

// BuildPath substitutes {name} placeholders in a path template using the values
// of the named flags. paramFlags maps the OpenAPI parameter name to the CLI
// flag name.
func BuildPath(cmd *cobra.Command, template string, paramFlags map[string]string) string {
	path := template
	for apiName, flagName := range paramFlags {
		v, _ := cmd.Flags().GetString(flagName)
		path = strings.ReplaceAll(path, "{"+apiName+"}", url.PathEscape(v))
	}
	return path
}

// AddBodyFlags registers the standard body flags on an operation command.
func AddBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String(FlagBody, "", "Request body as a JSON string")
	cmd.Flags().String(FlagBodyFile, "", "Read request body from a file (use '-' for stdin)")
}

// QStr adds a string query parameter if its flag was set.
func QStr(cmd *cobra.Command, q url.Values, apiName, flagName string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetString(flagName)
		q.Set(apiName, v)
	}
}

// QInt adds an integer query parameter if its flag was set.
func QInt(cmd *cobra.Command, q url.Values, apiName, flagName string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetInt64(flagName)
		q.Set(apiName, fmt.Sprintf("%d", v))
	}
}

// QNum adds a floating-point query parameter if its flag was set.
func QNum(cmd *cobra.Command, q url.Values, apiName, flagName string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetFloat64(flagName)
		q.Set(apiName, fmt.Sprintf("%v", v))
	}
}

// QBool adds a boolean query parameter if its flag was set.
func QBool(cmd *cobra.Command, q url.Values, apiName, flagName string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetBool(flagName)
		q.Set(apiName, fmt.Sprintf("%t", v))
	}
}

// QStrList adds a comma-joined list query parameter if its flag was set.
func QStrList(cmd *cobra.Command, q url.Values, apiName, flagName string) {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetStringSlice(flagName)
		q.Set(apiName, strings.Join(v, ","))
	}
}
