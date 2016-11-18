package handlers

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/gorouter/access_log/schema"
	"code.cloudfoundry.org/gorouter/proxy/handler"
	"code.cloudfoundry.org/gorouter/proxy/utils"
)

type requestHandler struct{}

// ServeHTTP creates the request handler object and adds it to the context
func (p *requestHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	proxyWriter := responseWriter.(utils.ProxyResponseWriter)

	alr := proxyWriter.Context().Value("AccessLogRecord")
	if alr == nil {
		p.logger.Error("AccessLogRecord not set on context", errors.New("failed-to-access-LogRecord"))
		return
	}
	accessLog := alr.(*schema.AccessLogRecord)

	handler := handler.NewRequestHandler(request, proxyWriter, p.reporter, accessLog, p.logger)
	proxyWriter.AddToContext("RequestHandler", handler)

	next(responseWriter, request)
}

// TODO: TEST THIS
func RetrieveRequestHandler(rspWr http.ResponseWriter) (handler.RequestHandler, error) {
	proxyWriter, ok := responseWriter.(utils.ProxyResponseWriter)
	if !ok {
		return nil, errors.New("response writer is not utils.ProxyResponseWriter")
	}

	h := proxyWriter.Context().Value("RequestHandler")
	if h == nil {
		return nil, errors.New("RequestHandler not set on context")
	}

	handler, ok := h.(*handler.RequestHandler)
	if !ok {
		return nil, errors.New("interface is not handler.RequestHandler")
	}

	return handler, nil
}
