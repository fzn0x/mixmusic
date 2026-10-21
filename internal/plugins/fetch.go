package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/sobek"
)

// fetchLimit bounds the body of a response plugins read.
const fetchLimit = 10 << 20

var fetchClient = &http.Client{Timeout: 30 * time.Second}

// fetchResult is a response, as fetch reads it beside the loop.
type fetchResult struct {
	status     int
	statusText string
	url        string
	headers    map[string]string
	body       []byte
}

// fetch is the web's fetch, for what plugins need of it: a request of a
// method, headers and a text body, whose response gives its status, its
// headers and its body, as text, JSON or bytes.
func (pl *plugin) fetch(call sobek.FunctionCall) sobek.Value {
	method, body := http.MethodGet, ""
	headers := map[string]string{}
	if opts, ok := call.Argument(1).(*sobek.Object); ok {
		if m := opts.Get("method"); m != nil && !sobek.IsUndefined(m) {
			method = strings.ToUpper(m.String())
		}
		if hs, ok := opts.Get("headers").(*sobek.Object); ok {
			for _, k := range hs.Keys() {
				headers[k] = hs.Get(k).String()
			}
		}
		if b := opts.Get("body"); b != nil && !sobek.IsUndefined(b) && !sobek.IsNull(b) {
			body = b.String()
		}
	}
	u, err := url.Parse(call.Argument(0).String())
	if err == nil && u.Scheme != "https" && u.Scheme != "http" {
		err = errors.New("fetch reads http and https addresses only")
	}
	return pl.async(func(ctx context.Context) (any, error) {
		if err != nil {
			return nil, err
		}
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := fetchClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		data, err := io.ReadAll(io.LimitReader(res.Body, fetchLimit+1))
		if err != nil {
			return nil, err
		}
		if len(data) > fetchLimit {
			return nil, fmt.Errorf("the response is larger than %d MB", fetchLimit>>20)
		}
		r := fetchResult{status: res.StatusCode, statusText: http.StatusText(res.StatusCode), url: res.Request.URL.String(),
			headers: map[string]string{}, body: data}
		for k, vs := range res.Header {
			r.headers[strings.ToLower(k)] = strings.Join(vs, ", ")
		}
		return r, nil
	}, func(v any) sobek.Value { return pl.response(v.(fetchResult)) })
}

// response is a fetch's response, as JavaScript sees it.
func (pl *plugin) response(r fetchResult) sobek.Value {
	vm := pl.vm
	settled := func(v sobek.Value, err error) sobek.Value {
		p, resolve, reject := vm.NewPromise()
		if err != nil {
			reject(vm.NewGoError(jsError(err)))
		} else {
			resolve(v)
		}
		return vm.ToValue(p)
	}
	o := vm.NewObject()
	o.Set("ok", r.status >= 200 && r.status < 300)
	o.Set("status", r.status)
	o.Set("statusText", r.statusText)
	o.Set("url", r.url)
	headers := vm.NewObject()
	headers.Set("get", func(name string) sobek.Value {
		if v, ok := r.headers[strings.ToLower(name)]; ok {
			return vm.ToValue(v)
		}
		return sobek.Null()
	})
	o.Set("headers", headers)
	o.Set("text", func() sobek.Value { return settled(vm.ToValue(string(r.body)), nil) })
	o.Set("json", func() sobek.Value {
		v, err := pl.parse(sobek.Undefined(), vm.ToValue(string(r.body)))
		return settled(v, err)
	})
	o.Set("arrayBuffer", func() sobek.Value { return settled(vm.ToValue(vm.NewArrayBuffer(r.body)), nil) })
	return o
}
