package prawxxey

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/common-go/cache"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

const (
	defaultCacheTTL            = 5 * time.Minute
	defaultMaxResponseBodySize = 100 << 20 // 100MB
)

// hop-by-hop headers (RFC 2616 section 13.5.1).
//
//nolint:gochecknoglobals
var hopByHopHeaders = map[string]struct{}{
	aichteeteapee.HeaderNameConnection:         {},
	aichteeteapee.HeaderNameKeepAlive:          {},
	aichteeteapee.HeaderNameProxyAuthenticate:  {},
	aichteeteapee.HeaderNameProxyAuthorization: {},
	aichteeteapee.HeaderNameTE:                 {},
	aichteeteapee.HeaderNameTrailer:            {},
	aichteeteapee.HeaderNameTrailers:           {},
	aichteeteapee.HeaderNameTransferEncoding:   {},
	aichteeteapee.HeaderNameUpgrade:            {},
}

type ForwardConfig struct {
	HTTPClient             *http.Client
	Cache                  cache.Cache
	CacheTTL               time.Duration
	CacheKeyFn             func(p *RequestPayload) string
	CacheKeyExcludeHeaders map[string]struct{}
	MaxResponseBodySize    int64
}

func ForwardRequest(
	ctx context.Context,
	cfg ForwardConfig,
	payload *RequestPayload,
) (*ResponseResult, error) {
	if ctx == nil {
		return nil, ctxerrors.Wrap(errNilContext, "forward request")
	}

	if payload == nil {
		return nil, ctxerrors.Wrap(errNilRequestPayload, "forward request")
	}

	if cfg.MaxResponseBodySize < 0 ||
		cfg.MaxResponseBodySize == math.MaxInt64 {
		return nil, ctxerrors.Wrap(errInvalidResponseLimit, "forward request")
	}

	logger := ctxscope.GetLogger(ctx)

	logger.Debug(
		"forwarding request",
		"upstream_method", payload.Method,
	)

	if cfg.Cache != nil {
		return forwardWithCache(ctx, cfg, payload)
	}

	result, err := doUpstreamRequest(
		ctx,
		cfg.HTTPClient,
		payload,
		responseBodyLimit(cfg),
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "forward uncached request")
	}

	return result, nil
}

func forwardWithCache(
	ctx context.Context,
	cfg ForwardConfig,
	payload *RequestPayload,
) (*ResponseResult, error) {
	if !isCacheableRequest(cfg, payload) {
		maxSize := responseBodyLimit(cfg)

		result, err := doUpstreamRequest(
			ctx,
			cfg.HTTPClient,
			payload,
			maxSize,
		)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "forward uncacheable request")
		}

		return result, nil
	}

	key := cacheKey(cfg, payload)
	if result, ok := tryCache(ctx, cfg, key); ok {
		return result, nil
	}

	maxSize := responseBodyLimit(cfg)

	result, err := doUpstreamRequest(
		ctx,
		cfg.HTTPClient,
		payload,
		maxSize,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "fetch response for cache")
	}

	storeInCache(ctx, cfg, key, result)
	setCacheStatus(result, aichteeteapee.CacheStatusMiss)

	return result, nil
}

func tryCache(
	ctx context.Context,
	cfg ForwardConfig,
	key string,
) (*ResponseResult, bool) {
	logger := ctxscope.GetLogger(ctx)

	data, err := cfg.Cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, cache.ErrCacheMiss) {
			logger.Debug("cache miss", "reason", "entry_not_found")

			return nil, false
		}

		logger.Warn(
			"cache read failed, continuing without cached response",
			"reason", "cache_read_failed",
			"err", err,
		)

		return nil, false
	}

	var result ResponseResult

	if jsonErr := json.Unmarshal(
		data, &result,
	); jsonErr != nil {
		logger.Warn(
			"cache entry corrupt",
			"reason", "cache_decode_failed",
			"err", jsonErr,
		)

		return nil, false
	}

	if int64(len(result.Body)) > responseBodyLimit(cfg) ||
		!isCacheableResponse(&result) {
		logger.Warn(
			"cached response rejected",
			"reason", "cache_policy_failed",
		)

		return nil, false
	}

	logger.Debug(
		"cache hit",
		"status_code", result.StatusCode,
	)

	setCacheStatus(&result, aichteeteapee.CacheStatusHit)

	return &result, true
}

func storeInCache(
	ctx context.Context,
	cfg ForwardConfig,
	key string,
	result *ResponseResult,
) {
	logger := ctxscope.GetLogger(ctx)

	if !isCacheableResponse(result) {
		logger.Debug(
			"response is not cacheable",
			"reason", "response_cache_policy",
			"status_code", result.StatusCode,
		)

		return
	}

	ttl := cfg.CacheTTL
	if ttl == 0 {
		ttl = defaultCacheTTL
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		logger.Warn(
			"cache response encode failed, continuing without cache write",
			"reason", "cache_encode_failed",
			"err", err,
		)

		return
	}

	if err := cfg.Cache.Set(ctx, key, encoded, ttl); err != nil {
		logger.Warn(
			"cache write failed, continuing with upstream response",
			"reason", "cache_write_failed",
			"err", err,
		)

		return
	}

	logger.Debug(
		"cached response",
		"ttl_ms", ttl.Milliseconds(),
	)
}

func doUpstreamRequest(
	ctx context.Context,
	httpClient *http.Client,
	payload *RequestPayload,
	maxResponseBodySize int64,
) (*ResponseResult, error) {
	if httpClient == nil {
		return nil, ctxerrors.Wrap(errNilHTTPClient, "do upstream request")
	}

	start := time.Now()

	req, err := buildUpstreamReq(ctx, payload)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build upstream request")
	}

	resp, err := executeUpstreamRequest(
		ctx,
		httpClient,
		req,
		payload.Method,
		start,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "do request")
	}

	defer closeResponseBody(ctx, resp.Body)

	respBody, err := io.ReadAll(
		io.LimitReader(resp.Body, maxResponseBodySize+1),
	)
	if err != nil {
		return nil, ctxerrors.Wrap(
			err, "read response body",
		)
	}

	if int64(len(respBody)) > maxResponseBodySize {
		return nil, ctxerrors.Wrap(
			errResponseBodyTooLarge,
			"read response body",
		)
	}

	logUpstreamResponse(
		ctx,
		resp.StatusCode,
		len(respBody),
		time.Since(start),
	)

	return &ResponseResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
	}, nil
}

func executeUpstreamRequest(
	ctx context.Context,
	httpClient *http.Client,
	request *http.Request,
	method string,
	startedAt time.Time,
) (*http.Response, error) {
	response, err := httpClient.Do(request)
	if err != nil {
		ctxscope.GetLogger(ctx).Error(
			"upstream request failed",
			"upstream_method", method,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"err", aichteeteapee.ErrAPIError,
		)

		return nil, ctxerrors.Wrap(
			sanitizeUpstreamError(err),
			"execute upstream request",
		)
	}

	return response, nil
}

func buildUpstreamReq(
	ctx context.Context,
	payload *RequestPayload,
) (*http.Request, error) {
	parsedURL, err := validateUpstreamDestination(payload.URL)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "validate upstream request")
	}

	var bodyReader io.Reader
	if len(payload.Body) > 0 {
		bodyReader = bytes.NewReader(payload.Body)
	}

	req, err := http.NewRequestWithContext(
		ctx, payload.Method,
		parsedURL.String(), bodyReader,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(
			err, "create request",
		)
	}

	if len(payload.Body) > 0 {
		req.ContentLength = int64(
			len(payload.Body),
		)
	}

	setUpstreamHeaders(req, payload)

	return req, nil
}

func validateUpstreamDestination(
	rawURL string,
) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, ctxerrors.Wrap(errInvalidUpstreamURL, "parse upstream URL")
	}

	if (parsedURL.Scheme != aichteeteapee.SchemeHTTP &&
		parsedURL.Scheme != aichteeteapee.SchemeHTTPS) ||
		parsedURL.Host == "" || parsedURL.User != nil {
		return nil, ctxerrors.Wrap(
			errInvalidUpstreamURL,
			"validate upstream URL",
		)
	}

	return parsedURL, nil
}

func logUpstreamResponse(
	ctx context.Context,
	statusCode, bodySize int,
	duration time.Duration,
) {
	logger := ctxscope.GetLogger(ctx).With(
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"body_size", bodySize,
	)

	if statusCode >= http.StatusInternalServerError {
		logger.Warn("upstream returned server error")

		return
	}

	logger.Debug("upstream response received")
}

func closeResponseBody(ctx context.Context, body io.Closer) {
	if err := body.Close(); err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"upstream response body close failed",
			"err", err,
		)
	}
}

func setCacheStatus(result *ResponseResult, status string) {
	if result.Headers == nil {
		result.Headers = make(map[string][]string)
	}

	result.Headers[aichteeteapee.HeaderNameXCacheStatus] = []string{status}
}

func setUpstreamHeaders(
	req *http.Request,
	payload *RequestPayload,
) {
	connectionHeaders := connectionHeaderNames(payload.Headers)

	for key, vals := range payload.Headers {
		canonKey := http.CanonicalHeaderKey(key)
		if _, skip := hopByHopHeaders[canonKey]; skip {
			continue
		}

		if _, skip := connectionHeaders[canonKey]; skip {
			continue
		}

		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}

	if payload.ClientIP != "" {
		req.Header.Set(
			aichteeteapee.HeaderNameXForwardedFor,
			payload.ClientIP,
		)
		req.Header.Set(
			aichteeteapee.HeaderNameXRealIP,
			payload.ClientIP,
		)
	}

	if payload.Proto != "" {
		req.Header.Set(
			aichteeteapee.HeaderNameXForwardedProto,
			payload.Proto,
		)
	}
}

func connectionHeaderNames(headers map[string][]string) map[string]struct{} {
	names := make(map[string]struct{})

	for key, values := range headers {
		if !strings.EqualFold(key, aichteeteapee.HeaderNameConnection) {
			continue
		}

		for _, value := range values {
			for token := range strings.SplitSeq(value, ",") {
				name := http.CanonicalHeaderKey(strings.TrimSpace(token))
				if name != "" {
					names[name] = struct{}{}
				}
			}
		}
	}

	return names
}

func RequestScheme(r *http.Request) string {
	if r.TLS != nil {
		return aichteeteapee.SchemeHTTPS
	}

	if proto := r.Header.Get(
		aichteeteapee.HeaderNameXForwardedProto,
	); proto != "" {
		return proto
	}

	return aichteeteapee.SchemeHTTP
}

func WriteError(
	w http.ResponseWriter,
	status int,
) {
	aichteeteapee.WriteJSON(
		w,
		status,
		aichteeteapee.ErrorResponse{
			Code: aichteeteapee.ErrorCodeFromHTTPStatus(
				status,
			),
			Message: http.StatusText(status),
		},
	)
}

func cacheKey(
	cfg ForwardConfig,
	payload *RequestPayload,
) string {
	if cfg.CacheKeyFn != nil {
		return cfg.CacheKeyFn(payload)
	}

	return payload.HashExcluding(
		cfg.CacheKeyExcludeHeaders,
	)
}

func responseBodyLimit(cfg ForwardConfig) int64 {
	if cfg.MaxResponseBodySize > 0 {
		return cfg.MaxResponseBodySize
	}

	return defaultMaxResponseBodySize
}

func isCacheableRequest(cfg ForwardConfig, payload *RequestPayload) bool {
	if payload.Method != http.MethodGet && payload.Method != http.MethodHead {
		return false
	}

	if cfg.HTTPClient != nil && cfg.HTTPClient.Jar != nil {
		return false
	}

	for name, values := range payload.Headers {
		if headerBypassesCache(name, values) {
			return false
		}
	}

	return true
}

func headerBypassesCache(name string, values []string) bool {
	switch {
	case strings.EqualFold(name, aichteeteapee.HeaderNameAuthorization),
		strings.EqualFold(name, aichteeteapee.HeaderNameCookie):
		return true
	case strings.EqualFold(name, aichteeteapee.HeaderNameCacheControl):
		return hasCacheDirective(values, "no-cache", "no-store")
	case strings.EqualFold(name, aichteeteapee.HeaderNamePragma):
		return hasCacheDirective(values, "no-cache")
	default:
		return false
	}
}

func isCacheableResponse(result *ResponseResult) bool {
	if result.StatusCode < http.StatusOK ||
		result.StatusCode >= http.StatusMultipleChoices {
		return false
	}

	for name, values := range result.Headers {
		if strings.EqualFold(name, aichteeteapee.HeaderNameSetCookie) ||
			strings.EqualFold(name, aichteeteapee.HeaderNameVary) {
			return false
		}

		if !strings.EqualFold(name, aichteeteapee.HeaderNameCacheControl) {
			continue
		}

		if hasCacheDirective(values, "no-cache", "no-store", "private") {
			return false
		}
	}

	return true
}

func hasCacheDirective(values []string, blocked ...string) bool {
	for _, value := range values {
		for directive := range strings.SplitSeq(value, ",") {
			name, _, _ := strings.Cut(directive, "=")
			name = strings.TrimSpace(name)

			for _, blockedName := range blocked {
				if strings.EqualFold(name, blockedName) {
					return true
				}
			}
		}
	}

	return false
}

func sanitizeUpstreamError(err error) error {
	for {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}

		err = urlError.Err
	}
}
