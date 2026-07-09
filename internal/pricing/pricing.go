package pricing

import "strings"

type Rate struct {
	InputPerMTok       float64
	CachedInputPerMTok float64
	CacheWritePerMTok  float64
	OutputPerMTok      float64
}

type Catalog map[string]Rate

func DefaultCatalog() Catalog {
	return Catalog{
		"gpt-5.6-sol":   {InputPerMTok: 5.00, CachedInputPerMTok: 0.50, CacheWritePerMTok: 6.25, OutputPerMTok: 30.00},
		"gpt-5.6-terra": {InputPerMTok: 2.50, CachedInputPerMTok: 0.25, CacheWritePerMTok: 3.125, OutputPerMTok: 15.00},
		"gpt-5.6-luna":  {InputPerMTok: 1.00, CachedInputPerMTok: 0.10, CacheWritePerMTok: 1.25, OutputPerMTok: 6.00},
		"gpt-5.5":       {InputPerMTok: 5.00, CachedInputPerMTok: 0.50, OutputPerMTok: 30.00},
		"gpt-5.4":       {InputPerMTok: 2.50, CachedInputPerMTok: 0.25, OutputPerMTok: 15.00},
		"gpt-5.4-mini":  {InputPerMTok: 0.75, CachedInputPerMTok: 0.075, OutputPerMTok: 4.50},
		"gpt-5":         {InputPerMTok: 1.25, CachedInputPerMTok: 0.125, OutputPerMTok: 10.00},
		"gpt-5-mini":    {InputPerMTok: 0.25, CachedInputPerMTok: 0.025, OutputPerMTok: 2.00},
		"gpt-5-nano":    {InputPerMTok: 0.05, CachedInputPerMTok: 0.005, OutputPerMTok: 0.40},
	}
}

func (c Catalog) EstimateUSD(model string, input, cachedInput, cacheWrite, output int64) float64 {
	rate, ok := c[model]
	if !ok {
		rate = c.matchPrefix(model)
	}
	billableInput := input - cachedInput
	if rate.CacheWritePerMTok > 0 {
		billableInput -= cacheWrite
	}
	if billableInput < 0 {
		billableInput = 0
	}
	return (float64(billableInput)/1_000_000)*rate.InputPerMTok +
		(float64(cachedInput)/1_000_000)*rate.CachedInputPerMTok +
		(float64(cacheWrite)/1_000_000)*rate.CacheWritePerMTok +
		(float64(output)/1_000_000)*rate.OutputPerMTok
}

func (c Catalog) matchPrefix(model string) Rate {
	bestKey := ""
	var best Rate
	for key, rate := range c {
		if strings.HasPrefix(model, key) && len(key) > len(bestKey) {
			bestKey = key
			best = rate
		}
	}
	return best
}
