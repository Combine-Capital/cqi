package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/protobuf/proto"
	"resty.dev/v3"
)

// Response wraps the resty response with CQI-specific error mapping
// and protobuf deserialization support.
type Response struct {
	resty      *resty.Response
	statusCode int
	headers    http.Header
	body       []byte
	err        error
}

// newResponse creates a new Response from a resty response and error.
// It maps HTTP status codes to CQI error types.
func newResponse(resp *resty.Response, err error) (*Response, error) {
	// Handle request errors
	if err != nil {
		return nil, mapRequestError(err)
	}

	// Read body if not already read
	var body []byte
	if resp.Body != nil {
		var readErr error
		body, readErr = io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, errors.NewTemporary("failed to read response body", readErr)
		}
	}

	// Create response wrapper
	response := &Response{
		resty:      resp,
		statusCode: resp.StatusCode(),
		headers:    resp.Header(),
		body:       body,
	}

	// Check for error status codes and map to CQI error types
	if err := mapStatusCodeToError(resp.StatusCode(), string(body)); err != nil {
		response.err = err
		return response, err
	}

	return response, nil
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int {
	return r.statusCode
}

// Status returns the HTTP status string (e.g., "200 OK").
func (r *Response) Status() string {
	if r.resty != nil {
		return r.resty.Status()
	}
	return http.StatusText(r.statusCode)
}

// Headers returns all response headers.
func (r *Response) Headers() http.Header {
	return r.headers
}

// Header returns the value of a single header.
func (r *Response) Header(key string) string {
	return r.headers.Get(key)
}

// Body returns the raw response body as bytes.
func (r *Response) Body() []byte {
	return r.body
}

// BodyAsString returns the response body as a string.
func (r *Response) BodyAsString() string {
	return string(r.body)
}

// BodyAsJSON unmarshals the response body into the provided struct.
func (r *Response) BodyAsJSON(dest interface{}) error {
	if r.err != nil {
		return r.err
	}

	if len(r.body) == 0 {
		return errors.NewInvalidInput("body", "empty response body")
	}

	if err := json.Unmarshal(r.body, dest); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON response")
	}

	return nil
}

// BodyAsProto unmarshals the response body as a protobuf message.
func (r *Response) BodyAsProto(msg proto.Message) error {
	if r.err != nil {
		return r.err
	}

	if len(r.body) == 0 {
		return errors.NewInvalidInput("body", "empty response body")
	}

	if err := proto.Unmarshal(r.body, msg); err != nil {
		return errors.Wrap(err, "failed to unmarshal protobuf response")
	}

	return nil
}

// Error returns the error associated with this response, if any.
func (r *Response) Error() error {
	return r.err
}

// IsSuccess returns true if the status code is 2xx.
func (r *Response) IsSuccess() bool {
	return r.statusCode >= 200 && r.statusCode < 300
}

// IsError returns true if the status code is 4xx or 5xx.
func (r *Response) IsError() bool {
	return r.statusCode >= 400
}

// mapRequestError maps request-level errors to CQI error types.
func mapRequestError(err error) error {
	// Context cancellation or timeout
	if err == context.Canceled || err == context.DeadlineExceeded {
		return errors.Wrap(err, "request canceled or timed out")
	}

	// Default to temporary error for network issues
	return errors.NewTemporary("request failed", err)
}

// mapStatusCodeToError maps HTTP status codes to CQI error types.
func mapStatusCodeToError(statusCode int, body string) error {
	// 2xx - success
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	// 3xx - redirects (handled by client automatically)
	if statusCode >= 300 && statusCode < 400 {
		return nil
	}

	// Create error message
	errMsg := fmt.Sprintf("HTTP %d: %s", statusCode, http.StatusText(statusCode))
	if len(body) > 0 && len(body) < 200 {
		errMsg = fmt.Sprintf("%s - %s", errMsg, body)
	}

	// 4xx - client errors
	switch statusCode {
	case http.StatusBadRequest: // 400
		return errors.NewInvalidInput("request", errMsg)
	case http.StatusUnauthorized: // 401
		return errors.NewUnauthorized(errMsg)
	case http.StatusForbidden: // 403
		return errors.NewUnauthorized(errMsg)
	case http.StatusNotFound: // 404
		return errors.NewNotFound("resource", errMsg)
	case http.StatusConflict: // 409
		return errors.NewPermanent(errMsg, nil)
	case http.StatusTooManyRequests: // 429
		return errors.NewTemporary(errMsg, nil)
	default:
		if statusCode >= 400 && statusCode < 500 {
			return errors.NewPermanent(errMsg, nil)
		}
	}

	// 5xx - server errors (temporary, should retry)
	if statusCode >= 500 {
		return errors.NewTemporary(errMsg, nil)
	}

	// Unknown status code
	return errors.NewPermanent(errMsg, nil)
}
