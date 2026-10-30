# Mapricot for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/mapricothq/mapricot-go.svg)](https://pkg.go.dev/github.com/mapricothq/mapricot-go)

The official Go client for the [Mapricot API](https://mapricot.com/): address and place search, reverse geocoding, nearby places, routing, and IP geolocation, with 250,000 free requests every month.

```sh
go get github.com/mapricothq/mapricot-go
```

It needs Go 1.22 or later and has no dependencies outside the standard library.

## Quick start

Create a server key in the [dashboard](https://app.mapricot.com/) and pass it to the client. Keep it out of your source code, for example by reading it from an environment variable as here.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mapricothq/mapricot-go"
)

func main() {
	client := mapricot.NewClient(os.Getenv("MAPRICOT_KEY"))

	places, err := client.Geocoding.Search(context.Background(), "128 SW 3rd Ave, Portland", &mapricot.SearchOptions{Limit: 1})
	if err != nil {
		log.Fatal(err)
	}
	for _, place := range places.Results {
		fmt.Println(place.Label, place.Lat, place.Lon)
	}
}
```

## Methods

Methods are grouped by product, the same way as the API's paths. Each returns a typed struct with the fields in the [API reference](https://mapricot.com/api-reference/). Options can be `nil`, and their zero values are left out of the request.

| Method | Endpoint |
| --- | --- |
| `client.Geocoding.Search(ctx, q, *SearchOptions)` | [`GET /geocoding/v1/search`](https://mapricot.com/docs/geocoding/#find-an-address) |
| `client.Geocoding.SearchBatch(ctx, []SearchQuery)` | [`POST /geocoding/v1/search/batch`](https://mapricot.com/docs/geocoding/#search-a-list-of-addresses) |
| `client.Geocoding.Autocomplete(ctx, q, *SearchOptions)` | [`GET /geocoding/v1/autocomplete`](https://mapricot.com/docs/geocoding/#suggest-as-people-type) |
| `client.Geocoding.Reverse(ctx, lat, lon, *ReverseOptions)` | [`GET /geocoding/v1/reverse`](https://mapricot.com/docs/geocoding/#find-the-address-at-a-point) |
| `client.Places.Nearby(ctx, lat, lon, *NearbyOptions)` | [`GET /places/v1/nearby`](https://mapricot.com/docs/places/#list-places-around-a-point) |
| `client.Places.Get(ctx, id, *PlaceOptions)` | [`GET /places/v1/{id}`](https://mapricot.com/docs/places/#look-up-a-saved-place) |
| `client.Routing.Route(ctx, waypoints, *RouteOptions)` | [`POST /routing/v1/route`](https://mapricot.com/docs/routing/#get-a-route) |
| `client.Routing.Isochrone(ctx, location, minutes, *IsochroneOptions)` | [`POST /routing/v1/isochrone`](https://mapricot.com/docs/routing/#get-an-isochrone) |
| `client.IPGeolocation.Lookup(ctx, ip)` | [`GET /ip-geolocation/v1/{ip}`](https://mapricot.com/docs/ip-geolocation/#look-up-an-address) |
| `client.Account(ctx)` | `GET /account`, which checks a key and is never billed |

```go
nearby, err := client.Places.Nearby(ctx, 45.52, -122.68, &mapricot.NearbyOptions{Category: []string{"food:coffee_shop"}, Radius: 1000})
routes, err := client.Routing.Route(ctx, []mapricot.Point{{Lat: 45.5189, Lon: -122.6793}, {Lat: 45.5214, Lon: -122.6828}}, &mapricot.RouteOptions{Mode: "walk"})
```

`SearchBatch` runs up to 100 searches in one request. Each `SearchQuery` has `Search`'s parameters, and `Responses` answers them in the same order, each with `Results` and `Attribution`, or an `Error` with the `Code` and `Message` that search would get, which isn't returned as an error. Each completed search is billed as one request; errors aren't.

```go
batch, err := client.Geocoding.SearchBatch(ctx, []mapricot.SearchQuery{{Q: "128 SW 3rd Ave, Portland", Limit: 1}, {Q: "10 Downing Street, London", Limit: 1}})
for _, response := range batch.Responses {
	if response.Error != nil {
		fmt.Println(response.Error.Message)
	} else {
		fmt.Println(response.Results[0].Label)
	}
}
```

A route's path comes as an encoded `Polyline` with five decimal places, which polyline decoders assume. Set `Geometries: "geojson"` to get it as a GeoJSON `LineString` in `Geometry` instead, which maps draw without decoding, or `Geometries: "polyline6"` for six decimal places.

Set `Steps: true` and each leg's `Steps` has its turn-by-turn directions: an `Instruction`, its distance and time, and its `Lat` and `Lon`. They're in English unless you set `Lang`, such as `Lang: "fr"`.

An isochrone is a GeoJSON `FeatureCollection` with one polygon per time, largest first, so a map that draws them in order shows the smaller ones on top. Each `Geometry.Coordinates` is the polygon's raw GeoJSON coordinates:

```go
area, err := client.Routing.Isochrone(ctx, mapricot.Point{Lat: 45.5189, Lon: -122.6793}, []int{5, 10}, &mapricot.IsochroneOptions{Mode: "walk"})
```

If your app shows results, show the credits in each response's `Attribution`.

## Errors and retries

An error response returns a `*mapricot.Error`, with the HTTP `Status`, the API's error `Code`, a `Message`, and the `RequestID`:

```go
_, err := client.Geocoding.Search(ctx, "", nil)
var apiErr *mapricot.Error
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Status, apiErr.Code) // 400 invalid_request
}
```

The client retries `503` responses and `429 concurrency_exceeded`, up to twice, waiting longer each time. Errors aren't billed. Change `client.MaxRetries` and `client.HTTPClient` to change this. The [API reference](https://mapricot.com/api-reference/#errors) lists every status.

## Links

- [Mapricot](https://mapricot.com/)
- [Go docs](https://mapricot.com/docs/go/)
- [API reference](https://mapricot.com/api-reference/)
- [Pricing](https://mapricot.com/pricing/)
