package carbon

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const electricityMapsBaseURL = "https://api.electricitymaps.com"

type electricityMapsResponse struct {
	Zone            string    `json:"zone"`
	Datetime        time.Time `json:"datetime"`
	CarbonIntensity float64   `json:"carbonIntensity"`
}

// ElectricityMapsClient fetches real-time carbon intensity from ElectricityMaps (global, API key).
type ElectricityMapsClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewElectricityMapsClient creates a client for ElectricityMaps API.
func NewElectricityMapsClient(apiKey string) *ElectricityMapsClient {
	return &ElectricityMapsClient{
		baseURL: electricityMapsBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetIntensity returns carbon intensity for a cloud region.
func (c *ElectricityMapsClient) GetIntensity(ctx context.Context, region string) (*Intensity, error) {
	zone, ok := RegionMapping[region]
	if !ok {
		zone = region
	}
	return c.GetIntensityForZone(ctx, zone)
}

// GetIntensityForZone returns carbon intensity for a zone code (e.g. US-MIDA-PJM).
func (c *ElectricityMapsClient) GetIntensityForZone(ctx context.Context, zone string) (*Intensity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v3/carbon-intensity/latest?zone="+zone, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("auth-token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch carbon intensity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid API key: ElectricityMaps requires ECOSCALE_CARBON_API_KEY")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("electricity maps API returned %d", resp.StatusCode)
	}

	var data electricityMapsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Intensity{
		Region:    zone,
		Value:     math.Round(data.CarbonIntensity*10) / 10,
		Unit:      "gCO2/kWh",
		Timestamp: data.Datetime.UTC(),
	}, nil
}

// CompareRegions compares carbon intensity between two regions.
func (c *ElectricityMapsClient) CompareRegions(ctx context.Context, regionA, regionB string) (*RegionComparison, error) {
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
