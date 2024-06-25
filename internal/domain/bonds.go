package domain

import "sort"

// InterestRateMap contains a mapping of interest rates at
// varying durations (months) from a given day
type InterestRateMap struct {
	Rates map[int]float64
}

func (im InterestRateMap) GetRate(months int) float64 {
	v, ok := im.Rates[months]
	if ok {
		return v
	}

	keys := []int{}
	for k := range im.Rates {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	// figure out closest values and interpolate
	if months < keys[0] {
		return im.Rates[keys[0]]
	}
	if months > keys[len(keys)-1] {
		return im.Rates[keys[len(keys)-1]]
	}

	for i := 0; i < len(keys)-1; i++ {
		key1 := keys[i]
		key2 := keys[i+1]
		if months > key1 && months < key2 {
			return (im.Rates[key1] + im.Rates[key2]) / 2
		}
	}

	panic("unable to compute rate")
}
