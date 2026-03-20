package carbon

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const carbonIntensityBaseURL = "https://api.carbonintensity.org.uk"

// UK region shortnames from Carbon Intensity API
var ukRegionMapping = map[string]string{
	"eu-west-2": "London",  // London
	"eu-west-1": "GB",      // Ireland - use GB as proxy (UK API has no IE)
	"eu-north-1": "GB",     // Sweden - use GB as proxy
}

// carbonIntensityResponse matches the API response structure.
type carbonIntensityResponse struct {
	Data []struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Regions []struct {
			RegionID   int    `json:"regionid"`
			Shortname  string `json:"shortname"`
			Intensity  struct {
				Forecast int    `json:"forecast"`
				Index    string `json:"index"`
			} `json:"intensity"`
		} `json:"regions"`
	} `json:"data"`
}

// CarbonIntensityClient fetches real-time carbon intensity from CarbonIntensity.org.uk (free, UK zones).
type CarbonIntensityClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCarbonIntensityClient creates a client for CarbonIntensity.org.uk.
func NewCarbonIntensityClient() *CarbonIntensityClient {
	return &CarbonIntensityClient{
		baseURL: carbonIntensityBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetIntensity returns carbon intensity for a cloud region.
func (c *CarbonIntensityClient) GetIntensity(ctx context.Context, region string) (*Intensity, error) {
	zone, ok := ukRegionMapping[region]
	if !ok {
		zone, ok = RegionMapping[region]
		if !ok {
			zone = region
		}
	}
	return c.GetIntensityForZone(ctx, zone)
}

// GetIntensityForZone returns carbon intensity for a zone (e.g. GB, London).
func (c *CarbonIntensityClient) GetIntensityForZone(ctx context.Context, zone string) (*Intensity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/regional", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch carbon intensity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("carbon intensity API returned %d", resp.StatusCode)
	}

	var data carbonIntensityResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(data.Data) == 0 || len(data.Data[0].Regions) == 0 {
		return nil, fmt.Errorf("no regional data")
	}

	// Find matching region (zone can be "GB", "London", etc.)
	var value float64
	found := false
	for _, r := range data.Data[0].Regions {
		if r.Shortname == zone {
			value = float64(r.Intensity.Forecast)
			found = true
			break
		}
	}
	if !found {
		// Try GB as fallback
		for _, r := range data.Data[0].Regions {
			if r.Shortname == "GB" {
				value = float64(r.Intensity.Forecast)
				found = true
				break
			}
		}
	}
	if !found {
		value = float64(data.Data[0].Regions[0].Intensity.Forecast)
	}

	return &Intensity{
		Region:    zone,
		Value:     math.Round(value*10) / 10,
		Unit:      "gCO2/kWh",
		Timestamp: time.Now().UTC(),
	}, nil
}

// CompareRegions compares carbon intensity between two regions.
func (c *CarbonIntensityClient) CompareRegions(ctx context.Context, regionA, regionB string) (*RegionComparison, error) {
	intA, err := c.GetIntensity(ctx, regionA)
	if err != nil {
		return nil, fmt.Errorf("get intensity for %s: %w", regionA, err)
	}
	intB, err := c.GetIntensity(ctx, regionB)
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
