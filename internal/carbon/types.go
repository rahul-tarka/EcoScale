package carbon

import "time"

// Intensity represents carbon intensity data for a region (gCO2/kWh).
type Intensity struct {
	Region      string    `json:"region"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp"`
	Forecast    []ForecastPoint `json:"forecast,omitempty"`
}

// ForecastPoint represents a single forecast data point.
type ForecastPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64  `json:"value"`
}

// RegionMapping maps cloud provider regions to carbon intensity zones.
// CarbonIntensity.org.uk and ElectricityMaps use zone codes (e.g., "US-CAL-CISO").
var RegionMapping = map[string]string{
	"us-east-1":      "US-MIDA-PJM",   // Virginia - PJM
	"us-east-2":      "US-MIDW-MISO",  // Ohio - MISO
	"us-west-1":      "US-NW-PACW",    // California - CAISO
	"us-west-2":      "US-NW-PACW",    // Oregon - BPA (hydro-heavy)
	"eu-west-1":      "IE",            // Ireland
	"eu-central-1":   "DE",            // Germany
	"eu-north-1":     "SE",            // Sweden (hydro/nuclear)
	"ap-northeast-1": "JP",            // Tokyo
	"ap-southeast-1": "SG",            // Singapore
}
