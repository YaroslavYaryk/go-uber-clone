package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ride-sharing/shared/types"

	tripTypes "ride-sharing/services/trip-service/pkg/types"
)

// mockRoundTripper lets tests intercept http.Get calls made inside GetRoute.
type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

// withTransport swaps http.DefaultTransport for the duration of fn, then restores it.
func withTransport(rt http.RoundTripper, fn func()) {
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	defer func() { http.DefaultTransport = orig }()
	fn()
}

// handlerTransport builds a RoundTripper that feeds requests into an http.HandlerFunc
// without any real networking.
func handlerTransport(h http.HandlerFunc) http.RoundTripper {
	return &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			h(rec, req)
			return rec.Result(), nil
		},
	}
}

func fp(f float64) *float64 { return &f }

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func osrmResponse(distance, duration float64, coords [][]float64) tripTypes.OsrmAPIResponse {
	return tripTypes.OsrmAPIResponse{
		Routes: []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		}{
			{
				Distance: distance,
				Duration: duration,
				Geometry: struct {
					Coordinates [][]float64 `json:"coordinates"`
				}{},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestGetRoute_Success(t *testing.T) {
	svc := NewService(nil)

	want := osrmResponse(1500.0, 240.0, [][]float64{{2.3522, 48.8566}, {2.3400, 48.8600}})

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(want)
	}

	withTransport(handlerTransport(handler), func() {
		pickup := types.Coordinate{Latitude: fp(48.8566), Longitude: fp(2.3522)}
		dest := types.Coordinate{Latitude: fp(48.8600), Longitude: fp(2.3400)}

		got, err := svc.GetRoute(context.Background(), &pickup, &dest)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Routes) != 1 {
			t.Fatalf("expected 1 route, got %d", len(got.Routes))
		}
		if got.Routes[0].Distance != want.Routes[0].Distance {
			t.Errorf("distance: want %f, got %f", want.Routes[0].Distance, got.Routes[0].Distance)
		}
		if got.Routes[0].Duration != want.Routes[0].Duration {
			t.Errorf("duration: want %f, got %f", want.Routes[0].Duration, got.Routes[0].Duration)
		}
	})
}

func TestGetRoute_EmptyRoutes(t *testing.T) {
	svc := NewService(nil)

	empty := tripTypes.OsrmAPIResponse{}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(empty)
	}

	withTransport(handlerTransport(handler), func() {
		pickup := types.Coordinate{Latitude: fp(48.8566), Longitude: fp(2.3522)}
		dest := types.Coordinate{Latitude: fp(48.8600), Longitude: fp(2.3400)}

		got, err := svc.GetRoute(context.Background(), &pickup, &dest)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Routes) != 0 {
			t.Errorf("expected 0 routes, got %d", len(got.Routes))
		}
	})
}

func TestGetRoute_NetworkError(t *testing.T) {
	svc := NewService(nil)

	transport := &mockRoundTripper{
		fn: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		},
	}

	withTransport(transport, func() {
		pickup := types.Coordinate{Latitude: fp(48.8566), Longitude: fp(2.3522)}
		dest := types.Coordinate{Latitude: fp(48.8600), Longitude: fp(2.3400)}

		_, err := svc.GetRoute(context.Background(), &pickup, &dest)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to fetch route from osrm API") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestGetRoute_InvalidJSON(t *testing.T) {
	svc := NewService(nil)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json {{{"))
	}

	withTransport(handlerTransport(handler), func() {
		pickup := types.Coordinate{Latitude: fp(48.8566), Longitude: fp(2.3522)}
		dest := types.Coordinate{Latitude: fp(48.8600), Longitude: fp(2.3400)}

		_, err := svc.GetRoute(context.Background(), &pickup, &dest)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "failed to unmarshal response body") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestGetRoute_URLContainsCoordinates(t *testing.T) {
	svc := NewService(nil)

	var capturedURL string

	transport := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(rec).Encode(tripTypes.OsrmAPIResponse{})
			return rec.Result(), nil
		},
	}

	withTransport(transport, func() {
		pickup := types.Coordinate{Latitude: fp(48.8566), Longitude: fp(2.3522)}
		dest := types.Coordinate{Latitude: fp(48.8600), Longitude: fp(2.3400)}

		_, _ = svc.GetRoute(context.Background(), &pickup, &dest)
	})

	if !strings.Contains(capturedURL, "router.project-osrm.org") {
		t.Errorf("expected OSRM host in URL, got: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "overview=full") {
		t.Errorf("expected overview=full query param, got: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "geometries=geojson") {
		t.Errorf("expected geometries=geojson query param, got: %s", capturedURL)
	}
}
