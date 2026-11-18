package upnp

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxDescription bounds the size of a description MixMusic reads.
const maxDescription = 256 << 10

// description is a device description, as UPnP's Device Architecture has
// it: the names are matched without their namespace.
type description struct {
	URLBase string       `xml:"URLBase"`
	Device  deviceDetail `xml:"device"`
}

type deviceDetail struct {
	DeviceType   string          `xml:"deviceType"`
	FriendlyName string          `xml:"friendlyName"`
	Manufacturer string          `xml:"manufacturer"`
	ModelName    string          `xml:"modelName"`
	UDN          string          `xml:"UDN"`
	Services     []serviceDetail `xml:"serviceList>service"`
	Devices      []deviceDetail  `xml:"deviceList>device"`
}

type serviceDetail struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// errNotRenderer is why a description is left out: it tells of no media
// renderer with an AVTransport service whose control address is the
// renderer's own.
var errNotRenderer = errors.New("upnp: not a media renderer")

// describe fetches the description at location, of the device at host,
// and returns the media renderer it tells of.
func describe(ctx context.Context, client *http.Client, location, host string) (Renderer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return Renderer{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return Renderer{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Renderer{}, fmt.Errorf("upnp: the description answered HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxDescription))
	if err != nil {
		return Renderer{}, err
	}
	return parseDescription(body, location, host)
}

// parseDescription reads the media renderer of a description fetched at
// location, of the device at host: the root device, or the first device in
// it that is one. Control addresses are relative to its URLBase, else to
// location.
func parseDescription(body []byte, location, host string) (Renderer, error) {
	var d description
	if err := xml.Unmarshal(body, &d); err != nil {
		return Renderer{}, err
	}
	base, err := url.Parse(location)
	if err != nil {
		return Renderer{}, err
	}
	if d.URLBase != "" {
		if b, err := url.Parse(strings.TrimSpace(d.URLBase)); err == nil {
			base = b
		}
	}
	dev, ok := findRenderer(d.Device)
	if !ok {
		return Renderer{}, errNotRenderer
	}
	r := Renderer{ID: strings.TrimSpace(dev.UDN), Name: strings.TrimSpace(dev.FriendlyName),
		Model: strings.TrimSpace(dev.ModelName), Host: host, Location: location}
	for _, s := range dev.Services {
		typ := strings.TrimSpace(s.ServiceType)
		control := strings.TrimSpace(s.ControlURL)
		ref, err := url.Parse(control)
		if control == "" || err != nil {
			continue
		}
		abs := base.ResolveReference(ref).String()
		if !sameHost(abs, host) {
			continue
		}
		svc := Service{Type: typ, Control: abs}
		switch {
		case strings.HasPrefix(typ, avTransport) && r.AVTransport.Control == "":
			r.AVTransport = svc
		case strings.HasPrefix(typ, renderingControl) && r.RenderingControl.Control == "":
			r.RenderingControl = svc
		case strings.HasPrefix(typ, connectionManager) && r.ConnectionManager.Control == "":
			r.ConnectionManager = svc
		}
	}
	if r.AVTransport.Control == "" {
		return Renderer{}, errNotRenderer
	}
	if r.ID == "" {
		r.ID = location
	}
	if r.Name == "" {
		r.Name = host
	}
	return r, nil
}

// findRenderer returns d, or the first device in it, that is a media
// renderer with an AVTransport service.
func findRenderer(d deviceDetail) (deviceDetail, bool) {
	if strings.Contains(d.DeviceType, ":device:MediaRenderer:") {
		for _, s := range d.Services {
			if strings.HasPrefix(strings.TrimSpace(s.ServiceType), avTransport) {
				return d, true
			}
		}
	}
	for _, c := range d.Devices {
		if r, ok := findRenderer(c); ok {
			return r, true
		}
	}
	return deviceDetail{}, false
}
