// Package locations implements privacy-aware location handling.
//
// Location modes, from least to most precise:
//
//	none            – nothing is recorded
//	area_only       – free-text area (district, village, shelter name)
//	approximate     – coordinates rounded to the instance precision
//	                  (default 2 decimals, about 1.1 km in latitude)
//	protected_exact – exact coordinates and/or address, encrypted at rest and
//	                  revealed only to authorized, audited viewers; a rounded
//	                  approximation is derived for overview purposes
package locations

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

// Mode is a location privacy mode.
type Mode string

const (
	ModeNone           Mode = "none"
	ModeAreaOnly       Mode = "area_only"
	ModeApproximate    Mode = "approximate"
	ModeProtectedExact Mode = "protected_exact"
)

// Modes lists all valid modes.
var Modes = []string{string(ModeNone), string(ModeAreaOnly), string(ModeApproximate), string(ModeProtectedExact)}

// MaxApproxDecimals is the finest precision allowed for approximate
// coordinates. Anything finer must use protected_exact.
const MaxApproxDecimals = 3

// Input is the client-supplied location.
type Input struct {
	Mode      string         `json:"mode"`
	AreaLabel string         `json:"area_label"`
	Lat       *float64       `json:"lat"`
	Lon       *float64       `json:"lon"`
	Exact     *ExactLocation `json:"exact,omitempty"`
	// KeepExact keeps a previously stored exact location on update without
	// the client having to reveal and resend it.
	KeepExact bool `json:"keep_exact,omitempty"`
}

// ExactLocation is the protected part of a location. It is only ever stored
// encrypted.
type ExactLocation struct {
	Lat        *float64 `json:"lat,omitempty"`
	Lon        *float64 `json:"lon,omitempty"`
	Address    string   `json:"address,omitempty"`
	Directions string   `json:"directions,omitempty"`
}

// Normalized is a validated location ready for storage.
type Normalized struct {
	Mode       Mode
	AreaLabel  string
	ApproxLat  *float64
	ApproxLon  *float64
	Decimals   *int16
	Exact      *ExactLocation
	ExactBytes []byte
	// KeepExact signals that the caller should retain the existing protected
	// exact location (and its derived approximation).
	KeepExact bool
}

// Round rounds v to the given number of decimals.
func Round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

func validCoord(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// Normalize validates in and applies privacy rules. decimals is the instance
// precision for approximate coordinates.
func Normalize(in Input, decimals int, errs validation.Errors) Normalized {
	if decimals < 0 {
		decimals = 0
	}
	if decimals > MaxApproxDecimals {
		decimals = MaxApproxDecimals
	}
	mode := Mode(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = ModeNone
	}
	errs.OneOf("location.mode", string(mode), Modes)
	area := validation.CleanText(in.AreaLabel)
	errs.Text("location.area_label", area, 0, 160, false)

	n := Normalized{Mode: mode}
	d := int16(decimals)
	switch mode {
	case ModeNone:
		// Intentionally discard anything else that was sent.
	case ModeAreaOnly:
		if area == "" {
			errs.Add("location.area_label", "An area (district, village, shelter name) is required.")
		}
		n.AreaLabel = area
	case ModeApproximate:
		n.AreaLabel = area
		if in.Lat == nil || in.Lon == nil || !validCoord(*in.Lat, *in.Lon) {
			errs.Add("location.lat", "Valid approximate coordinates are required.")
			break
		}
		lat, lon := Round(*in.Lat, decimals), Round(*in.Lon, decimals)
		n.ApproxLat, n.ApproxLon, n.Decimals = &lat, &lon, &d
	case ModeProtectedExact:
		n.AreaLabel = area
		ex := in.Exact
		if ex == nil && in.KeepExact {
			n.KeepExact = true
			break
		}
		if ex == nil {
			errs.Add("location.exact", "An exact address or coordinates are required.")
			break
		}
		clean := ExactLocation{
			Address:    validation.CleanText(ex.Address),
			Directions: validation.CleanText(ex.Directions),
		}
		errs.Text("location.exact.address", clean.Address, 0, 300, true)
		errs.Text("location.exact.directions", clean.Directions, 0, 500, true)
		hasCoords := ex.Lat != nil && ex.Lon != nil
		if hasCoords {
			if !validCoord(*ex.Lat, *ex.Lon) {
				errs.Add("location.exact.lat", "Coordinates are out of range.")
				break
			}
			lat, lon := *ex.Lat, *ex.Lon
			clean.Lat, clean.Lon = &lat, &lon
			alat, alon := Round(lat, decimals), Round(lon, decimals)
			n.ApproxLat, n.ApproxLon, n.Decimals = &alat, &alon, &d
		}
		if !hasCoords && clean.Address == "" {
			errs.Add("location.exact", "An exact address or coordinates are required.")
			break
		}
		b, err := json.Marshal(clean)
		if err != nil {
			errs.Add("location.exact", "Invalid exact location.")
			break
		}
		n.Exact = &clean
		n.ExactBytes = b
	}
	return n
}

// View is the non-protected location representation returned by the API.
type View struct {
	Mode              Mode     `json:"mode"`
	AreaLabel         string   `json:"area_label"`
	ApproxLat         *float64 `json:"approx_lat"`
	ApproxLon         *float64 `json:"approx_lon"`
	PrecisionDecimals *int16   `json:"precision_decimals"`
	HasExact          bool     `json:"has_exact"`
}
