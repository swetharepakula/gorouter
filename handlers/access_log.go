package handlers

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/gorouter/access_log"
	"code.cloudfoundry.org/gorouter/access_log/schema"
	"code.cloudfoundry.org/gorouter/proxy/utils"

	"github.com/urfave/negroni"
)

type accessLog struct {
	accessLogger      access_log.AccessLogger
	extraHeadersToLog *[]string
	pool              sync.Pool
}

func NewAccessLog(accessLogger access_log.AccessLogger, extraHeadersToLog *[]string) negroni.Handler {
	return &accessLog{
		accessLogger:      accessLogger,
		extraHeadersToLog: extraHeadersToLog,
		pool: sync.Pool{
			New: func() interface{} {
				return &schema.AccessLogRecord{}
			},
		},
	}
}

func (a *accessLog) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	proxyWriter := rw.(utils.ProxyResponseWriter)
	alr := a.pool.Get().(*schema.AccessLogRecord)
	defer a.pool.Put(alr)
	alr.Request = r
	alr.StartedAt = time.Now()
	alr.ExtraHeadersToLog = a.extraHeadersToLog

	requestBodyCounter := &countingReadCloser{delegate: r.Body}
	r.Body = requestBodyCounter

	proxyWriter.AddToContext("AccessLogRecord", alr)

	next(rw, r)

	alr.RequestBytesReceived = requestBodyCounter.GetCount()
	alr.BodyBytesSent = proxyWriter.Size()
	alr.FinishedAt = time.Now()
	a.accessLogger.Log(*alr)
}

type countingReadCloser struct {
	delegate io.ReadCloser
	count    uint32
}

func (crc *countingReadCloser) Read(b []byte) (int, error) {
	n, err := crc.delegate.Read(b)
	atomic.AddUint32(&crc.count, uint32(n))
	return n, err
}

func (crc *countingReadCloser) GetCount() int {
	return int(atomic.LoadUint32(&crc.count))
}

func (crc *countingReadCloser) Close() error {
	return crc.delegate.Close()
}
