package runtime

import (
	"context"
	"net/http"
)

// WrapCtxReqRespErr wraps a handler of shape func(context.Context, Req) (Resp, error)
// into an http.Handler. It binds the request, calls the handler, and writes the response.
func WrapCtxReqRespErr[Req, Resp any](
	handler func(context.Context, Req) (Resp, error),
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := Bind(r, &req); err != nil {
			RespondError(w, err)
			return
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			RespondError(w, err)
			return
		}

		RespondJSON(w, http.StatusOK, resp)
	})
}

// WrapCtxReqErr wraps a handler of shape func(context.Context, Req) error
// into an http.Handler. It binds the request, calls the handler, and writes 204 No Content on success.
func WrapCtxReqErr[Req any](
	handler func(context.Context, Req) error,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := Bind(r, &req); err != nil {
			RespondError(w, err)
			return
		}

		if err := handler(r.Context(), req); err != nil {
			RespondError(w, err)
			return
		}

		RespondNoContent(w)
	})
}

// WrapCtxRespErr wraps a handler of shape func(context.Context) (Resp, error)
// into an http.Handler. It calls the handler and writes the response.
func WrapCtxRespErr[Resp any](
	handler func(context.Context) (Resp, error),
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler(r.Context())
		if err != nil {
			RespondError(w, err)
			return
		}

		RespondJSON(w, http.StatusOK, resp)
	})
}

// WrapCtxErr wraps a handler of shape func(context.Context) error
// into an http.Handler. It calls the handler and writes 204 No Content on success.
func WrapCtxErr(handler func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(r.Context()); err != nil {
			RespondError(w, err)
			return
		}

		RespondNoContent(w)
	})
}
