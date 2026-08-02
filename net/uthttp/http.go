package uthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"gitlab.com/goxp/cloud0/logger"
)

const (
	JSONContentType ContentType = iota
	XMLContentType
	FormUrlencodedContentType
)

var (
	ErrUnmarshalResponse = errors.New("failed to unmarshal response")
)

// HTTPRequest represents an HTTP request.
type HTTPRequest struct {
	Method string            // HTTP method
	URL    string            // URL to send the request to
	Header map[string]string // HTTP headers
	Body   interface{}       // Body of the request
	LogTag string            // Tag to use for logging
	Silent bool              // If true, skip info-level request/response logs
}

type HTTPResponse[T any] struct {
	StatusCode int
	Header     map[string][]string
	Body       T
}

type Config struct {
	Timeout time.Duration
}

type Options struct {
	DisallowUnknownFields bool // This field is removed, only here for backward compalibility
	ContentType           ContentType
	ContentLength         int64
}

type ContentType int

func NewHTTPClient(cfg Config) *http.Client {
	client := &http.Client{Timeout: cfg.Timeout}
	return client
}

func DefaultOptions() Options {
	return Options{
		DisallowUnknownFields: false,
		ContentType:           JSONContentType,
		ContentLength:         0,
	}
}

func WithJSONContentType() func(o *Options) {
	return func(o *Options) {
		o.ContentType = JSONContentType
	}
}

func WithXMLContentType() func(o *Options) {
	return func(o *Options) {
		o.ContentType = XMLContentType
	}
}

func WithFormUrlencodedContentType() func(o *Options) {
	return func(o *Options) {
		o.ContentType = FormUrlencodedContentType
	}
}

func WithContentLength(length int64) func(o *Options) {
	return func(o *Options) {
		o.ContentLength = length
	}
}

func SendHTTPRequest[T any](ctx context.Context, client *http.Client, httpReq HTTPRequest, opts Options, optFuncs ...func(*Options)) (HTTPResponse[T], error) {
	log := logger.WithCtx(ctx, httpReq.LogTag)

	for _, o := range optFuncs {
		o(&opts)
	}

	var res HTTPResponse[T]
	var reqReader io.Reader
	var bodyBytes []byte
	var err error
	// Marshal request body
	if httpReq.Body != nil {
		var ok bool
		reqReader, ok = httpReq.Body.(io.Reader)
		if !ok {
			switch opts.ContentType {
			case XMLContentType:
				bodyBytes, err = xml.Marshal(httpReq.Body)
			case FormUrlencodedContentType:
				bodyBytes, err = structToURLValues(httpReq.Body)
			default:
				bodyBytes, err = json.Marshal(httpReq.Body)
			}
			if err != nil {
				log.WithError(err).Error("error marshaling request body")
				return res, err
			}

			if !httpReq.Silent {
				log.Infof("api: %v header: %v requestData: %v", httpReq.URL, httpReq.Header, string(bodyBytes))
			}
			reqReader = bytes.NewReader(bodyBytes)
		}
	}

	// Create HTTP request
	req, err := http.NewRequest(httpReq.Method, httpReq.URL, reqReader)
	if err != nil {
		log.WithError(err).Error("error creating HTTP request")
		return res, err
	}

	// Add headers
	for k, v := range httpReq.Header {
		req.Header.Set(k, v)
	}

	if req.Header.Get("Content-Type") == "" {
		switch opts.ContentType {
		case XMLContentType:
			req.Header.Set("Content-Type", "application/xml")
		case FormUrlencodedContentType:
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		default:
			req.Header.Set("Content-Type", "application/json")
		}
	}

	req.ContentLength = opts.ContentLength

	// Send HTTP request
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("error sending HTTP request")
		return res, err
	}
	defer resp.Body.Close()
	res.StatusCode = resp.StatusCode
	res.Header = resp.Header

	var buf bytes.Buffer
	respReader := io.TeeReader(resp.Body, &buf)

	var body T
	switch opts.ContentType {
	case XMLContentType:
		dec := xml.NewDecoder(respReader)
		err = dec.Decode(&body)
	default:
		dec := json.NewDecoder(respReader)
		if opts.DisallowUnknownFields {
			dec.DisallowUnknownFields()
		}
		err = dec.Decode(&body)
	}
	if err != nil {
		if !errors.Is(err, io.EOF) || buf.Len() != 0 {
			log.WithField("response", res).WithError(err).Errorf("api: %v error unmarshalling response", httpReq.URL)
			return res, fmt.Errorf("%w: %w", ErrUnmarshalResponse, err)
		}
	}

	if !httpReq.Silent {
		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != http.StatusAccepted {
			log.Infof("api: %v statusCode: %v responseData: %v", httpReq.URL, resp.StatusCode, buf.String())
		} else {
			log.Infof("api: %v statusCode: %v responseData: %+v", httpReq.URL, resp.StatusCode, body)
		}
	}

	res.Body = body

	return res, nil
}

func MakeURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	path = strings.TrimPrefix(path, "/")

	return fmt.Sprintf("%s/%s", baseURL, path)
}

func structToURLValues(data interface{}) ([]byte, error) {
	values := url.Values{}

	v := reflect.ValueOf(data)
	t := v.Type()

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			tag := field.Tag.Get("form")
			if tag != "" {
				values.Set(tag, v.Field(i).String())
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			values.Set(fmt.Sprintf("%v", key), fmt.Sprintf("%v", v.MapIndex(key)))
		}
	default:
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}
	return []byte(values.Encode()), nil
}
