package mapricot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func live(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("MAPRICOT_KEY")
	if key == "" {
		t.Skip("set MAPRICOT_KEY to run tests against the live API")
	}
	return NewClient(key)
}

func TestSearch(t *testing.T) {
	places, err := live(t).Search(context.Background(), "128 SW 3rd Ave, Portland", &SearchOptions{Country: []string{"US"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if places.Results[0].Address.Locality != "Portland" || len(places.Attribution) == 0 {
		t.Errorf("got %+v", places)
	}
}

func TestAutocomplete(t *testing.T) {
	places, err := live(t).Autocomplete(context.Background(), "stumptown", &SearchOptions{Focus: &Point{45.52, -122.68}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(places.Results[0].Name, "Stumptown") {
		t.Errorf("got %+v", places.Results[0])
	}
}

func TestReverse(t *testing.T) {
	places, err := live(t).Reverse(context.Background(), 45.5217, -122.6733, &ReverseOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if places.Results[0].DistanceMeters == nil {
		t.Errorf("no distance in %+v", places.Results[0])
	}
}

func TestNearby(t *testing.T) {
	places, err := live(t).Nearby(context.Background(), 45.52, -122.68, &NearbyOptions{Category: []string{"food:coffee_shop"}, Radius: 1000, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(places.Results[0].Categories, "food:coffee_shop") {
		t.Errorf("got %+v", places.Results[0])
	}
}

func TestPlace(t *testing.T) {
	client := live(t)
	places, err := client.Search(context.Background(), "Pioneer Courthouse Square, Portland", &SearchOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	place, err := client.Place(context.Background(), places.Results[0].ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if place.Result.ID != places.Results[0].ID {
		t.Errorf("got %+v", place.Result)
	}
}

func TestRoute(t *testing.T) {
	routes, err := live(t).Route(context.Background(), []Point{{43.7396, 7.4275}, {43.734, 7.4222}}, &RouteOptions{Mode: "walk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes.Routes[0].Legs) != 1 || routes.Routes[0].Polyline == "" {
		t.Errorf("got %+v", routes)
	}
}

func TestRouteGeoJSON(t *testing.T) {
	routes, err := live(t).Route(context.Background(), []Point{{43.7396, 7.4275}, {43.734, 7.4222}}, &RouteOptions{Geometries: "geojson"})
	if err != nil {
		t.Fatal(err)
	}
	if geometry := routes.Routes[0].Geometry; geometry == nil || geometry.Type != "LineString" || len(geometry.Coordinates) < 2 {
		t.Errorf("got %+v", routes)
	}
}

func TestIP(t *testing.T) {
	location, err := live(t).IP(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	if !location.Found || location.CountryCode != "US" {
		t.Errorf("got %+v", location)
	}
}

func TestAccount(t *testing.T) {
	account, err := live(t).Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.Status != "active" || account.CreatedAt.IsZero() {
		t.Errorf("got %+v", account)
	}
}

func TestInvalidRequest(t *testing.T) {
	_, err := live(t).Search(context.Background(), "", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Code != "invalid_request" || apiErr.RequestID == "" {
		t.Errorf("got %v", err)
	}
}

func TestBadKey(t *testing.T) {
	_, err := NewClient("mapricot_sk_bad").Account(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 401 || apiErr.Code != "unauthorized" {
		t.Errorf("got %v", err)
	}
}

type failure struct {
	status int
	code   string
}

func stub(t *testing.T, failures ...failure) (*Client, *[]string) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		w.Header().Set("X-Request-Id", "r1")
		if len(seen) > len(failures) {
			w.Write([]byte(`{"id":"a1"}`))
			return
		}
		f := failures[len(seen)-1]
		w.WriteHeader(f.status)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": f.code, "message": f.code}, "requestId": "r1"})
	}))
	t.Cleanup(server.Close)
	client := NewClient("k")
	client.BaseURL = server.URL
	return client, &seen
}

func TestRetries(t *testing.T) {
	client, seen := stub(t, failure{503, "origin_overloaded"}, failure{429, "concurrency_exceeded"})
	if _, err := client.Search(context.Background(), "x", &SearchOptions{Focus: &Point{1, 2}, Country: []string{"US", "CA"}}); err != nil {
		t.Fatal(err)
	}
	if len(*seen) != 3 || (*seen)[0] != "/v1/search?country=US%2CCA&focus=1%2C2&q=x" {
		t.Errorf("requests %v", *seen)
	}
}

func TestNoRetryWhenOutOfCredit(t *testing.T) {
	client, seen := stub(t, failure{429, "credit_exhausted"})
	_, err := client.Account(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "credit_exhausted" || apiErr.RequestID != "r1" || len(*seen) != 1 {
		t.Errorf("got %v after %d requests", err, len(*seen))
	}
}
