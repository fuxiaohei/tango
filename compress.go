package tango

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"net"
	"net/http"
	"bufio"
	"strings"
)

const (
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLength   = "Content-Length"
	HeaderContentType     = "Content-Type"
	HeaderVary            = "Vary"
)

type Compress struct {
	exts map[string]bool
}
 
func NewCompress(exts []string) *Compress {
	compress := &Compress{make(map[string]bool)}
	for _, ext:=range exts {
		compress.exts[strings.ToLower(ext)] = true
	}
	return compress
}

func (compress *Compress) Handle(ctx *Context) {
	ae := ctx.Req().Header.Get("Accept-Encoding")
	if ae == "" {
		ctx.Next()
		return
	}

	if len(compress.exts) > 0 {
		if _, ok := compress.exts[strings.ToLower(path.Ext(ctx.Req().URL.Path))]; !ok {
			ctx.Next()
			return
		}
	}

	// for cache server
	ctx.Header().Add("Vary", "Accept-Encoding")

	splitted := strings.SplitN(ae, ",", -1)
	var writer io.Writer
	for _, val := range splitted {
		val = strings.TrimSpace(val)
		if val == "gzip" {
			ctx.Header().Set("Content-Encoding", "gzip")
			writer, _ = gzip.NewWriterLevel(ctx, gzip.BestSpeed)
			break
		} else if val == "deflate" {
			ctx.Header().Set("Content-Encoding", "deflate")
			writer, _ = flate.NewWriter(ctx, flate.BestSpeed)
			break
		}
	}

	if writer == nil {
		ctx.Next()
		return
	}

	gzw := compressWriter{writer, ctx.ResponseWriter}
	ctx.ResponseWriter = gzw

	ctx.Next()

	// delete content length after we know we have been written to
	gzw.Header().Del("Content-Length")

	switch writer.(type) {
		case *gzip.Writer:
			writer.(*gzip.Writer).Close()
		case *flate.Writer:
			writer.(*flate.Writer).Close()
	}
}

type compressWriter struct {
	w io.Writer
	ResponseWriter
}

func (grw compressWriter) Write(p []byte) (int, error) {
	if len(grw.Header().Get(HeaderContentType)) == 0 {
		grw.Header().Set(HeaderContentType, http.DetectContentType(p))
	}

	return grw.w.Write(p)
}

func (grw compressWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := grw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}
