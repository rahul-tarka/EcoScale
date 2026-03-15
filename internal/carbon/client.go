package carbon

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Client defines the interface for fetching carbon intensity data.
// Implementations can use CarbonIntensity.org.uk, ElectricityMaps, or mock data.
type Client interface {
	GetIntensity(ctx context.Context, region string) (*Intensity, error)
	GetIntensityForZone(ctx context.Context, zone string) (*Intensity, error)
	CompareRegions(ctx context.Context, regionA, regionB string) (*RegionComparison, error)
}

// RegionComparison holds the result of comparing two regions' carbon intensity.
type RegionComparison struct {
	RegionA       string
	RegionB       string
	IntensityA    float64
	IntensityB    float64
	GreenerRegion string
	SavingsPercent float64
	Timestamp     time.Time
}

// MockClient provides deterministic mock data for development and testing.
// Simulates regional variations: us-west-2 (hydro) < us-east-1 (fossil-heavy).
type MockClient struct {
	mu sync.RWMutex
	// Base intensities (gCO2/kWh) - realistic values for different grid mixes
	baseIntensities map[string]float64
	// Optional time-of-day modifier (solar peaks at noon = lower intensity)
	useTimeModifier bool
}

// NewMockClient creates a MockClient with realistic default values.
func NewMockClient(useTimeModifier bool) *MockClient {
	return &MockClient{
		useTimeModifier: useTimeModifier,
		baseIntensities: map[string]float64{
			"us-east-1":      420, // PJM - coal/gas heavy
			"us-east-2":      380,
			"us-west-1":      280, // CA - solar/wind
			"us-west-2":      180, // Oregon - hydro dominant (greenest)
			"eu-west-1":      320,
			"eu-central-1":   350,
			"eu-north-1":     45, // Sweden - nuclear/hydro
			"ap-northeast-1": 450,
			"ap-southeast-1": 420,
			// Zone codes (fallback)
			"US-MIDA-PJM":   420,
			"US-NW-PACW":    180,
			"IE":            320,
			"DE":            350,
			"SE":            45,
		},
	}
}

// GetIntensity returns carbon intensity for a cloud region.
func (m *MockClient) GetIntensity(ctx context.Context, region string) (*Intensity, error) {
	zone, ok := RegionMapping[region]
	if !ok {
		zone = region
	}
	return m.GetIntensityForZone(ctx, zone)
}

// GetIntensityForZone returns carbon intensity for a zone code.
func (m *MockClient) GetIntensityForZone(ctx context.Context, zone string) (*Intensity, error) {
	m.mu.RLock()
	base, ok := m.baseIntensities[zone]
	m.mu.RUnlock()

	if !ok {
		// Try region mapping in reverse
		for r, z := range RegionMapping {
			if z == zone {
				m.mu.RLock()
				base = m.baseIntensities[z]
				m.mu.RUnlock()
				if base == 0 {
					base = 300 // default
				}
				return &Intensity{
					Region:    r,
					Value:     base,
					Unit:      "gCO2/kWh",
					Timestamp: time.Now().UTC(),
				}, nil
			}
		}
		base = 300
	}

	value := base
	if m.useTimeModifier {
		hour := float64(time.Now().UTC().Hour())
		// Solar dips intensity by ~20% during 10am-4pm UTC
		if hour >= 10 && hour <= 16 {
			value = base * 0.8
		}
	}

	return &Intensity{
		Region:    zone,
		Value:     math.Round(value*10) / 10,
		Unit:      "gCO2/kWh",
		Timestamp: time.Now().UTC(),
	}, nil
}

// CompareRegions compares carbon intensity between two regions.
func (m *MockClient) CompareRegions(ctx context.Context, regionA, regionB string) (*RegionComparison, error) {
	intA, err := m.GetIntensity(ctx, regionA)
	if err != nil {
		return nil, fmt.Errorf("get intensity for %s: %w", regionA, err)
	}
	intB, err := m.GetIntensity(ctx, regionB)
	if err != nil {
		return nil, fmt.Errorf("get intensity for %s: %w", regionB, err)
	}

	greener := regionA
	savings := 0.0
	if intB.Value < intA.Value {
		greener = regionB
		if intA.Value > 0 {
			savings = (1 - intB.Value/intA.Value) * 100
		}
	} else if intA.Value > 0 {
		savings = (1 - intA.Value/intB.Value) * 100
	}

	return &RegionComparison{
		RegionA:        regionA,
		RegionB:        regionB,
		IntensityA:     intA.Value,
		IntensityB:     intB.Value,
		GreenerRegion:  greener,
		SavingsPercent: math.Round(savings*100) / 100,
		Timestamp:      time.Now().UTC(),
	}, nil
}
