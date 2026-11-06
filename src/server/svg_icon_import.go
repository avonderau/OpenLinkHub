package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// A bounded static SVG is rasterized to PNG before entering either LCD's
// existing image store. No SVG markup is served to clients or hardware.
const maxIconSetSVGBytes = 512 * 1024

var internalSVGReference = regexp.MustCompile(`url\(\s*['"]?#[A-Za-z0-9_-]+['"]?\s*\)`)

// Restrict inline CSS to inert SVG presentation rules. CSS escapes can
// disguise url() and @import from simple substring checks, so reject them
// before passing an uploaded SVG to the external renderer.
var safeIconSVGStyleProperties = map[string]bool{
	"fill": true, "fill-opacity": true, "fill-rule": true,
	"stroke": true, "stroke-width": true, "stroke-opacity": true,
	"stroke-linecap": true, "stroke-linejoin": true, "stroke-miterlimit": true,
	"stroke-dasharray": true, "stroke-dashoffset": true,
	"opacity": true, "color": true, "stop-color": true, "stop-opacity": true,
	"clip-path": true, "clip-rule": true,
	"font-size": true, "font-family": true, "font-weight": true,
	"text-anchor": true, "display": true,
}
var safeIconSVGAttributes = map[string]bool{
	"xmlns": true, "xlink": true, "version": true, "id": true, "class": true, "style": true,
	"viewbox": true, "preserveaspectratio": true,
	"x": true, "y": true, "width": true, "height": true, "x1": true, "y1": true,
	"x2": true, "y2": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true,
	"fx": true, "fy": true, "fr": true, "d": true, "points": true,
	"transform": true, "gradienttransform": true, "gradientunits": true,
	"offset": true, "spreadmethod": true,
	"fill": true, "fill-opacity": true, "fill-rule": true,
	"stroke": true, "stroke-width": true, "stroke-opacity": true,
	"stroke-linecap": true, "stroke-linejoin": true, "stroke-miterlimit": true,
	"stroke-dasharray": true, "stroke-dashoffset": true,
	"opacity": true, "color": true, "stop-color": true, "stop-opacity": true,
	"clip-path": true, "clip-rule": true, "vector-effect": true,
	"font-size": true, "font-family": true, "font-weight": true,
	"text-anchor": true, "display": true,
	"role": true, "lang": true, "space": true,
}

func validateIconSVGInlineStyle(value string) error {
	if len(value) > 4096 {
		return errors.New("SVG inline style is too large")
	}
	// An escaped identifier such as \75rl(...) must never bypass the resource
	// check. Comments, rulesets, directives, and !important are unnecessary
	// for a static icon and are not accepted.
	if strings.ContainsAny(value, "\\{}@!") ||
		strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return errors.New("SVG CSS escapes, rules or directives forbidden")
	}
	for _, declaration := range strings.Split(value, ";") {
		declaration = strings.TrimSpace(declaration)
		if declaration == "" {
			continue
		}
		key, propertyValue, ok := strings.Cut(declaration, ":")
		if !ok {
			return errors.New("invalid SVG CSS declaration")
		}
		key = strings.ToLower(strings.TrimSpace(key))
		propertyValue = strings.TrimSpace(propertyValue)
		if !safeIconSVGStyleProperties[key] || propertyValue == "" ||
			strings.Contains(propertyValue, ":") {
			return errors.New("unsupported SVG CSS presentation property or value")
		}
		if err := validateIconSVGResourceValue(propertyValue); err != nil {
			return err
		}
	}
	return nil
}
func validateIconSVGResourceValue(value string) error {
	lowered := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, "\\") ||
		strings.ContainsAny(value, "\x00") ||
		strings.Contains(lowered, "@import") ||
		strings.Contains(lowered, "javascript:") ||
		strings.Contains(lowered, "data:") ||
		strings.Contains(lowered, "expression(") {
		return errors.New("SVG CSS escapes or active content forbidden")
	}
	if strings.Contains(lowered, "url(") &&
		strings.Contains(internalSVGReference.ReplaceAllString(lowered, ""), "url(") {
		return errors.New("SVG external resources forbidden")
	}
	return nil
}

func svgConverterAvailable() bool { _, err := exec.LookPath("rsvg-convert"); return err == nil }

func validateIconSVG(data []byte) error {
	if len(data) == 0 || len(data) > maxIconSetSVGBytes {
		return errors.New("SVG exceeds 512 KiB")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth, count := 0, 0
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid SVG XML: %w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return errors.New("SVG DOCTYPE/entities are forbidden")
		case xml.ProcInst:
			if strings.ToLower(t.Target) != "xml" {
				return errors.New("SVG processing instructions are forbidden")
			}
		case xml.StartElement:
			depth++
			count++
			if depth > 24 || count > 2048 || (depth == 1 && t.Name.Local != "svg") {
				return errors.New("invalid or overly complex SVG")
			}
			// Do not let foreign namespaces impersonate an allowed SVG element.
			if t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg" {
				return errors.New("unsupported SVG XML namespace")
			}
			switch t.Name.Local {
			case "svg", "g", "path", "circle", "ellipse", "rect", "line", "polyline", "polygon", "defs", "linearGradient", "radialGradient", "stop", "clipPath", "title", "desc":
			default:
				return fmt.Errorf("unsupported SVG element %q", t.Name.Local)
			}
			if len(t.Attr) > 64 {
				return errors.New("too many SVG attributes")
			}
			for _, attr := range t.Attr {
				name := strings.ToLower(attr.Name.Local)
				if !safeIconSVGAttributes[name] || strings.HasPrefix(name, "on") {
					return fmt.Errorf("unsupported SVG attribute %q", name)
				}
				if name == "xmlns" || (attr.Name.Space == "xmlns" && name == "xlink") {
					if attr.Value != "http://www.w3.org/2000/svg" &&
						attr.Value != "http://www.w3.org/1999/xlink" {
						return errors.New("unsupported SVG namespace declaration")
					}
					continue
				}
				if attr.Name.Space != "" && attr.Name.Space != "http://www.w3.org/XML/1998/namespace" {
					return errors.New("unsupported SVG attribute namespace")
				}
				if name == "style" {
					if err := validateIconSVGInlineStyle(attr.Value); err != nil {
						return err
					}
					continue
				}
				if err := validateIconSVGResourceValue(attr.Value); err != nil {
					return err
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if len(t) > 8192 && len(strings.TrimSpace(string(t))) > 0 {
				return errors.New("oversized SVG text")
			}
		}
	}
	if depth != 0 || count == 0 {
		return errors.New("empty or unbalanced SVG")
	}
	return nil
}
func rasterizeIconSVG(data []byte) ([]byte, error) {
	if err := validateIconSVG(data); err != nil {
		return nil, err
	}
	binary, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return nil, errors.New("SVG import requires librsvg2-bin (rsvg-convert)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--format=png", "--width=128", "--height=128")
	cmd.Stdin = bytes.NewReader(data)
	png, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, errors.New("SVG conversion timed out")
	}
	if err != nil {
		return nil, fmt.Errorf("SVG conversion failed: %w", err)
	}
	if len(png) == 0 || len(png) > 512*1024 {
		return nil, errors.New("invalid rendered SVG size")
	}
	size, format, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil || format != "png" || size.Width < 1 || size.Width > 128 || size.Height < 1 || size.Height > 128 {
		return nil, errors.New("SVG did not produce bounded PNG")
	}
	return png, nil
}
