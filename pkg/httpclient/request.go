package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/protobuf/proto"
	"resty.dev/v3"
)

// Request represents an HTTP request with a fluent builder API.
// It wraps resty.Request with CQI-specific features like protobuf support
// and error mapping.
type Request struct {
	client *Client
	resty  *resty.Request
	ctx    context.Context
	method string
	url    string
}

// SetMethod sets the HTTP method for the request.
func (r *Request) SetMethod(method string) *Request {
	r.method = method
	return r
}

// SetURL sets the URL for the request.
// Can be relative (appended to BaseURL) or absolute.
func (r *Request) SetURL(url string) *Request {
	r.url = url
	return r
}

// WithHeader sets a single header on the request.
func (r *Request) WithHeader(key, value string) *Request {
	r.resty.SetHeader(key, value)
	return r
}

// WithHeaders sets multiple headers on the request.
func (r *Request) WithHeaders(headers map[string]string) *Request {
	r.resty.SetHeaders(headers)
	return r
}

// WithQuery adds a single query parameter to the request.
func (r *Request) WithQuery(key, value string) *Request {
	r.resty.SetQueryParam(key, value)
	return r
}

// WithQueryParams adds multiple query parameters to the request.
func (r *Request) WithQueryParams(params map[string]string) *Request {
	r.resty.SetQueryParams(params)
	return r
}

// WithJSON sets the request body as JSON.
// The body is automatically serialized to JSON.
func (r *Request) WithJSON(body interface{}) *Request {
	r.resty.SetBody(body)
	r.resty.SetHeader("Content-Type", "application/json")
	return r
}

// WithProto sets the request body as a protobuf message.
// The message is serialized to wire format before sending.
func (r *Request) WithProto(msg proto.Message) *Request {
	data, err := proto.Marshal(msg)
	if err != nil {
		// Store error to be returned during Do()
		r.resty.SetError(err)
		return r
	}
	r.resty.SetBody(data)
	r.resty.SetHeader("Content-Type", "application/x-protobuf")
	return r
}

// WithBody sets the request body as raw bytes.
func (r *Request) WithBody(body []byte) *Request {
	r.resty.SetBody(body)
	return r
}

// WithFormData sets the request body as form data.
func (r *Request) WithFormData(data map[string]string) *Request {
	r.resty.SetFormData(data)
	return r
}

// WithTimeout sets a custom timeout for this specific request.
// Overrides the client's default timeout.
func (r *Request) WithTimeout(timeout time.Duration) *Request {
	r.ctx, _ = context.WithTimeout(r.ctx, timeout)
	return r
}

// WithAuthToken sets the Bearer authentication token.
func (r *Request) WithAuthToken(token string) *Request {
	r.resty.SetAuthToken(token)
	return r
}

// WithBasicAuth sets basic authentication credentials.
func (r *Request) WithBasicAuth(username, password string) *Request {
	r.resty.SetBasicAuth(username, password)
	return r
}

// IntoJSON tells the request to unmarshal the response body into the provided struct.
func (r *Request) IntoJSON(result interface{}) *Request {
	r.resty.SetResult(result)
	return r
}

// IntoProto tells the request to unmarshal the response body as a protobuf message.
func (r *Request) IntoProto(msg proto.Message) *Request {
	// We'll handle protobuf deserialization in the response
	r.resty.SetDoNotParseResponse(true)
	return r
}

// Do executes the request and returns the response.
// It enforces rate limiting, maps errors to CQI error types,
// and handles retries automatically.
func (r *Request) Do() (*Response, error) {
	// Check rate limit
	if err := r.client.checkRateLimit(r.ctx); err != nil {
		return nil, err
	}

	// Set context on resty request
	r.resty.SetContext(r.ctx)

	// Execute request with the configured method and URL
	var resp *resty.Response
	var err error

	switch r.method {
	case http.MethodGet:
		resp, err = r.resty.Get(r.url)
	case http.MethodPost:
		resp, err = r.resty.Post(r.url)
	case http.MethodPut:
		resp, err = r.resty.Put(r.url)
	case http.MethodPatch:
		resp, err = r.resty.Patch(r.url)
	case http.MethodDelete:
		resp, err = r.resty.Delete(r.url)
	case http.MethodHead:
		resp, err = r.resty.Head(r.url)
	case http.MethodOptions:
		resp, err = r.resty.Options(r.url)
	default:
		return nil, errors.NewPermanent(fmt.Sprintf("unsupported HTTP method: %s", r.method), nil)
	}

	// Wrap response and map errors
	return newResponse(resp, err)
}
