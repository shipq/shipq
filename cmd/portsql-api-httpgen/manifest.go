package main

// Manifest represents the discovered endpoints from a package.
type Manifest struct {
	Endpoints []ManifestEndpoint `json:"endpoints"`
}

// ManifestEndpoint represents a single discovered endpoint.
type ManifestEndpoint struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	HandlerPkg  string `json:"handler_pkg"`
	HandlerName string `json:"handler_name"`
	Shape       string `json:"shape"` // ctx_req_resp_err, ctx_req_err, ctx_resp_err, ctx_err
	ReqType     string `json:"req_type,omitempty"`
	RespType    string `json:"resp_type,omitempty"`
}
