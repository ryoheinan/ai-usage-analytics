package pricing

import "testing"

func TestEstimateUSDUsesCachedInputRate(t *testing.T) {
	catalog := Catalog{
		"test-model": {InputPerMTok: 2, CachedInputPerMTok: 0.5, OutputPerMTok: 8},
	}

	got := catalog.EstimateUSD("test-model", 1_000_000, 250_000, 0, 500_000)
	want := 5.625
	if got != want {
		t.Fatalf("EstimateUSD() = %v, want %v", got, want)
	}
}

func TestEstimateUSDUsesCacheWriteRate(t *testing.T) {
	catalog := Catalog{
		"test-model": {InputPerMTok: 2, CachedInputPerMTok: 0.5, CacheWritePerMTok: 2.5, OutputPerMTok: 8},
	}

	got := catalog.EstimateUSD("test-model", 1_000_000, 250_000, 100_000, 500_000)
	want := 5.675
	if got != want {
		t.Fatalf("EstimateUSD() = %v, want %v", got, want)
	}
}

func TestEstimateUSDTreatsCacheWriteAsInputWithoutCacheWriteRate(t *testing.T) {
	catalog := Catalog{
		"test-model": {InputPerMTok: 2, CachedInputPerMTok: 0.5, OutputPerMTok: 8},
	}

	got := catalog.EstimateUSD("test-model", 1_000_000, 250_000, 100_000, 500_000)
	want := 5.625
	if got != want {
		t.Fatalf("EstimateUSD() = %v, want %v", got, want)
	}
}
