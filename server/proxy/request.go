package proxy

type RequestPayload struct {
	Method   string              `json:"method"`
	URL      string              `json:"url"`
	Headers  map[string][]string `json:"headers,omitempty"`
	Body     []byte              `json:"body,omitempty"`
	ClientIP string              `json:"clientIp,omitempty"`
	Proto    string              `json:"proto,omitempty"`
}

type ResponseResult struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       []byte              `json:"body,omitempty"`
}
