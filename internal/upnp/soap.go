package upnp

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// maxAnswer bounds the size of an action's answer MixMusic reads.
const maxAnswer = 256 << 10

// arg is an argument of an action, in the order the service declares it.
type arg struct{ name, value string }

// call sends the action of svc with args, and returns what it answers, by
// name. An action the renderer turned down is a *Fault.
func call(ctx context.Context, client *http.Client, svc Service, action string, args ...arg) (map[string]string, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	b.WriteString(`<u:` + action + ` xmlns:u="`)
	xml.EscapeText(&b, []byte(svc.Type))
	b.WriteString(`">`)
	for _, a := range args {
		b.WriteString("<" + a.name + ">")
		xml.EscapeText(&b, []byte(a.value))
		b.WriteString("</" + a.name + ">")
	}
	b.WriteString(`</u:` + action + `></s:Body></s:Envelope>`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, svc.Control, &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+svc.Type+"#"+action+`"`)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAnswer))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		if f, ok := parseFault(body); ok {
			f.Action = action
			return nil, f
		}
		return nil, fmt.Errorf("upnp: %s: the renderer answered HTTP %d", action, res.StatusCode)
	}
	return parseAnswer(body)
}

// parseAnswer reads the arguments of an action's answer: the elements in
// the first element of the envelope's body.
func parseAnswer(body []byte) (map[string]string, error) {
	d := xml.NewDecoder(bytes.NewReader(body))
	out := map[string]string{}
	depth := 0
	inBody := false
	bodyDepth := 0
	var name string
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("upnp: unreadable answer: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case !inBody && t.Name.Local == "Body":
				inBody, bodyDepth = true, depth
			case inBody && depth == bodyDepth+2:
				name = t.Name.Local
				text.Reset()
			}
		case xml.CharData:
			if name != "" {
				text.Write(t)
			}
		case xml.EndElement:
			if inBody && depth == bodyDepth+2 && name != "" {
				out[name] = text.String()
				name = ""
			}
			if inBody && depth == bodyDepth {
				inBody = false
			}
			depth--
		}
	}
	return out, nil
}

// parseFault reads the UPnP error of a SOAP fault, wherever it is in the
// body.
func parseFault(body []byte) (*Fault, bool) {
	var f struct {
		Code        string `xml:"Body>Fault>detail>UPnPError>errorCode"`
		Description string `xml:"Body>Fault>detail>UPnPError>errorDescription"`
	}
	if err := xml.Unmarshal(body, &f); err != nil || strings.TrimSpace(f.Code) == "" {
		return nil, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(f.Code))
	if err != nil {
		return nil, false
	}
	return &Fault{Code: code, Description: strings.TrimSpace(f.Description)}, true
}
