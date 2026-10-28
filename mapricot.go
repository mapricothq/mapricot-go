// Package mapricot is the official client for the Mapricot API: address and place search, reverse geocoding, nearby
// places, routing, and IP geolocation. See https://mapricot.com/docs/go/.
package mapricot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const version = "0.7.0"

// Client calls the Mapricot API. Make one with NewClient. Its methods are grouped by product, as in
// client.Geocoding.Search. Its settings can be changed before first use.
type Client struct {
	APIKey  string
	BaseURL string
	// HTTPClient sends requests. Its timeout applies to each attempt.
	HTTPClient *http.Client
	// MaxRetries is how many times a 503 or a 429 with too many requests in progress is retried.
	MaxRetries int

	Geocoding     *GeocodingService
	Places        *PlacesService
	Routing       *RoutingService
	IPGeolocation *IPGeolocationService
}

// GeocodingService calls the /geocoding/v1 endpoints.
type GeocodingService struct{ client *Client }

// PlacesService calls the /places/v1 endpoints.
type PlacesService struct{ client *Client }

// RoutingService calls the /routing/v1 endpoints.
type RoutingService struct{ client *Client }

// IPGeolocationService calls the /ip-geolocation/v1 endpoints.
type IPGeolocationService struct{ client *Client }

// NewClient returns a client that authenticates with apiKey, such as os.Getenv("MAPRICOT_KEY").
func NewClient(apiKey string) *Client {
	c := &Client{
		APIKey:     apiKey,
		BaseURL:    "https://api.mapricot.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 2,
	}
	c.Geocoding = &GeocodingService{c}
	c.Places = &PlacesService{c}
	c.Routing = &RoutingService{c}
	c.IPGeolocation = &IPGeolocationService{c}
	return c
}

// Error is an error response from the API.
type Error struct {
	Status    int
	Code      string
	Message   string
	RequestID string
}

func (e *Error) Error() string {
	return fmt.Sprintf("mapricot: %d %s: %s", e.Status, e.Code, e.Message)
}

type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Attribution struct {
	Source string `json:"source"`
	Text   string `json:"text"`
	URL    string `json:"url"`
}

// Address holds the parts of a place's address that its source has. Missing parts are empty.
type Address struct {
	HouseNumber string `json:"houseNumber"`
	Street      string `json:"street"`
	PostalCode  string `json:"postalCode"`
	Locality    string `json:"locality"`
	Region      string `json:"region"`
	RegionCode  string `json:"regionCode"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

type Place struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Label      string   `json:"label"`
	Lat        float64  `json:"lat"`
	Lon        float64  `json:"lon"`
	Address    *Address `json:"address"`
	Categories []string `json:"categories"`
	Phones     []string `json:"phones"`
	Websites   []string `json:"websites"`
	Socials    []string `json:"socials"`
	Brand      string   `json:"brand"`
	// OpeningHours is in OpenStreetMap's opening_hours syntax, such as "Mo-Fr 08:00-18:00".
	OpeningHours string `json:"openingHours"`
	// DistanceMeters is set by reverse, nearby, and searches with a focus.
	DistanceMeters *int   `json:"distanceMeters"`
	Source         string `json:"source"`
}

type Places struct {
	Results     []Place       `json:"results"`
	Attribution []Attribution `json:"attribution"`
}

type PlaceResult struct {
	Result      Place         `json:"result"`
	Attribution []Attribution `json:"attribution"`
}

type Leg struct {
	DistanceMeters  int `json:"distanceMeters"`
	DurationSeconds int `json:"durationSeconds"`
	// Steps are the leg's turn-by-turn directions, in order, when RouteOptions.Steps is set. The last one is the
	// arrival.
	Steps []Step `json:"steps"`
}

// Step is one maneuver, at Lat and Lon on the route's path, and the stretch from it to the next.
type Step struct {
	Instruction     string  `json:"instruction"`
	DistanceMeters  int     `json:"distanceMeters"`
	DurationSeconds int     `json:"durationSeconds"`
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
}

type Route struct {
	DistanceMeters  int `json:"distanceMeters"`
	DurationSeconds int `json:"durationSeconds"`
	// Polyline is encoded with five decimal places, or six for the polyline6 geometries. It is empty for geojson.
	Polyline string `json:"polyline"`
	// Geometry is set for the geojson geometries.
	Geometry *LineString `json:"geometry"`
	Legs     []Leg       `json:"legs"`
}

// LineString is a GeoJSON line, with each point as [longitude, latitude].
type LineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// Routes is empty when no path connects the waypoints.
type Routes struct {
	Routes      []Route       `json:"routes"`
	Attribution []Attribution `json:"attribution"`
}

// Isochrones is a GeoJSON FeatureCollection with one area per time, largest first. Features is empty when there is
// no road near the location.
type Isochrones struct {
	Type        string        `json:"type"`
	Features    []Isochrone   `json:"features"`
	Attribution []Attribution `json:"attribution"`
}

type Isochrone struct {
	Type       string `json:"type"`
	Properties struct {
		Minutes int `json:"minutes"`
	} `json:"properties"`
	Geometry Polygon `json:"geometry"`
}

// Polygon is a GeoJSON Polygon or MultiPolygon. Coordinates holds its rings of [longitude, latitude] points as JSON,
// nested one level deeper for a MultiPolygon.
type Polygon struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type IPLocation struct {
	IP             string `json:"ip"`
	Found          bool   `json:"found"`
	ContinentCode  string `json:"continentCode"`
	CountryCode    string `json:"countryCode"`
	CountryName    string `json:"countryName"`
	RegionCode     string `json:"regionCode"`
	RegionName     string `json:"regionName"`
	City           string `json:"city"`
	Location       *Point `json:"location"`
	Source         string `json:"source"`
	AttributionURL string `json:"attributionUrl"`
	LicenseURL     string `json:"licenseUrl"`
	DatasetRelease string `json:"datasetRelease"`
}

type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// SearchOptions are optional parameters for GeocodingService.Search and Autocomplete. Zero values are left out.
type SearchOptions struct {
	Focus   *Point
	Country []string
	Limit   int
	Lang    string
}

// ReverseOptions are optional parameters for GeocodingService.Reverse. Zero values are left out.
type ReverseOptions struct {
	Limit int
	Lang  string
}

// NearbyOptions are optional parameters for PlacesService.Nearby. Zero values are left out.
type NearbyOptions struct {
	Category []string
	Radius   int
	Limit    int
	Lang     string
}

// PlaceOptions are optional parameters for PlacesService.Get. Zero values are left out.
type PlaceOptions struct {
	Lang string
}

// RouteOptions are optional parameters for RoutingService.Route. Mode is "drive" (the default), "walk", or "bicycle". Geometries
// is "polyline" (the default), "polyline6", or "geojson". Steps adds each leg's turn-by-turn steps, and Lang is their
// ISO 639-1 language, English by default.
type RouteOptions struct {
	Mode       string
	Geometries string
	Steps      bool
	Lang       string
}

// Search finds addresses and places matching free text.
func (s *GeocodingService) Search(ctx context.Context, q string, opts *SearchOptions) (*Places, error) {
	return get[Places](ctx, s.client, "/geocoding/v1/search", searchQuery(q, opts))
}

// Autocomplete suggests addresses and places as someone types.
func (s *GeocodingService) Autocomplete(ctx context.Context, q string, opts *SearchOptions) (*Places, error) {
	return get[Places](ctx, s.client, "/geocoding/v1/autocomplete", searchQuery(q, opts))
}

// Reverse finds the address nearest a coordinate, or the area around it.
func (s *GeocodingService) Reverse(ctx context.Context, lat, lon float64, opts *ReverseOptions) (*Places, error) {
	query := pointQuery(lat, lon)
	if opts != nil {
		setInt(query, "limit", opts.Limit)
		setString(query, "lang", opts.Lang)
	}
	return get[Places](ctx, s.client, "/geocoding/v1/reverse", query)
}

// Nearby lists places around a coordinate, optionally by category.
func (s *PlacesService) Nearby(ctx context.Context, lat, lon float64, opts *NearbyOptions) (*Places, error) {
	query := pointQuery(lat, lon)
	if opts != nil {
		setString(query, "category", strings.Join(opts.Category, ","))
		setInt(query, "radius", opts.Radius)
		setInt(query, "limit", opts.Limit)
		setString(query, "lang", opts.Lang)
	}
	return get[Places](ctx, s.client, "/places/v1/nearby", query)
}

// Get looks up one place by its Mapricot ID. An ID that no longer resolves returns an *Error with Status 404.
func (s *PlacesService) Get(ctx context.Context, id string, opts *PlaceOptions) (*PlaceResult, error) {
	query := url.Values{}
	if opts != nil {
		setString(query, "lang", opts.Lang)
	}
	return get[PlaceResult](ctx, s.client, "/places/v1/"+url.PathEscape(id), query)
}

// Route finds a route through 2 to 20 waypoints, visited in order.
func (s *RoutingService) Route(ctx context.Context, waypoints []Point, opts *RouteOptions) (*Routes, error) {
	body := struct {
		Waypoints  []Point `json:"waypoints"`
		Mode       string  `json:"mode,omitempty"`
		Geometries string  `json:"geometries,omitempty"`
		Steps      bool    `json:"steps,omitempty"`
		Lang       string  `json:"lang,omitempty"`
	}{Waypoints: waypoints}
	if opts != nil {
		body.Mode = opts.Mode
		body.Geometries = opts.Geometries
		body.Steps = opts.Steps
		body.Lang = opts.Lang
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var routes Routes
	if err := s.client.do(ctx, http.MethodPost, "/routing/v1/route", nil, encoded, &routes); err != nil {
		return nil, err
	}
	return &routes, nil
}

// IsochroneOptions are optional parameters for RoutingService.Isochrone. Mode is "drive" (the default), "walk", or "bicycle".
type IsochroneOptions struct {
	Mode string
}

// Isochrone returns the areas reachable from a location within 1 to 4 times, each 1 to 60 minutes.
func (s *RoutingService) Isochrone(ctx context.Context, location Point, minutes []int, opts *IsochroneOptions) (*Isochrones, error) {
	body := struct {
		Location Point  `json:"location"`
		Minutes  []int  `json:"minutes"`
		Mode     string `json:"mode,omitempty"`
	}{Location: location, Minutes: minutes}
	if opts != nil {
		body.Mode = opts.Mode
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var isochrones Isochrones
	if err := s.client.do(ctx, http.MethodPost, "/routing/v1/isochrone", nil, encoded, &isochrones); err != nil {
		return nil, err
	}
	return &isochrones, nil
}

// Lookup returns the approximate location of a public IP address.
func (s *IPGeolocationService) Lookup(ctx context.Context, ip string) (*IPLocation, error) {
	return get[IPLocation](ctx, s.client, "/ip-geolocation/v1/"+url.PathEscape(ip), nil)
}

// Account returns the key's account. It isn't billed, so it checks a key for free.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	return get[Account](ctx, c, "/account", nil)
}

func get[T any](ctx context.Context, c *Client, path string, query url.Values) (*T, error) {
	var result T
	if err := c.do(ctx, http.MethodGet, path, query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, result any) error {
	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; ; attempt++ {
		err := c.attempt(ctx, method, target, body, result)
		apiErr, ok := err.(*Error)
		if !ok || attempt >= c.MaxRetries || !retryable(apiErr) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond << attempt):
		}
	}
}

func (c *Client) attempt(ctx context.Context, method, target string, body []byte, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", "mapricot-go/"+version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	apiErr := &Error{Status: resp.StatusCode, Code: "unknown", Message: resp.Status, RequestID: resp.Header.Get("X-Request-Id")}
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.NewDecoder(resp.Body).Decode(&envelope) == nil && envelope.Error.Code != "" {
		apiErr.Code, apiErr.Message = envelope.Error.Code, envelope.Error.Message
	}
	return apiErr
}

func retryable(err *Error) bool {
	return err.Status == http.StatusServiceUnavailable || err.Code == "concurrency_exceeded"
}

func searchQuery(q string, opts *SearchOptions) url.Values {
	query := url.Values{"q": {q}}
	if opts != nil {
		if opts.Focus != nil {
			query.Set("focus", formatFloat(opts.Focus.Lat)+","+formatFloat(opts.Focus.Lon))
		}
		setString(query, "country", strings.Join(opts.Country, ","))
		setInt(query, "limit", opts.Limit)
		setString(query, "lang", opts.Lang)
	}
	return query
}

func pointQuery(lat, lon float64) url.Values {
	return url.Values{"lat": {formatFloat(lat)}, "lon": {formatFloat(lon)}}
}

func setString(query url.Values, name, value string) {
	if value != "" {
		query.Set(name, value)
	}
}

func setInt(query url.Values, name string, value int) {
	if value != 0 {
		query.Set(name, strconv.Itoa(value))
	}
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
