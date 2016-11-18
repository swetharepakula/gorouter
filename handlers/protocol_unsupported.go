package handlers

import "net/http"

type protocolUnsupportedHandler struct{}

// ServeHTTP creates the request handler object and adds it to the context
func (p *protocolUnsupportedHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	handler, err := RetrieveRequestHandler(responseWriter)
	if err != nil {
		// Write error response
	}

	if !isProtocolSupported(request) {
		handler.HandleUnsupportedProtocol()
		return
	}

	next(responseWriter, request)
}

func isProtocolSupported(request *http.Request) bool {
	return request.ProtoMajor == 1 && (request.ProtoMinor == 0 || request.ProtoMinor == 1)
}
