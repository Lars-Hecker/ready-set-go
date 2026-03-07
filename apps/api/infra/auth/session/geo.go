package session

type GeoInfo struct {
	CountryCode string
	City        string
}

type GeoLookup interface {
	Lookup(ip string) GeoInfo
}

type NoOpGeoLookup struct{}

func NewNoOpGeoLookup() *NoOpGeoLookup {
	return &NoOpGeoLookup{}
}

func (g *NoOpGeoLookup) Lookup(ip string) GeoInfo {
	return GeoInfo{}
}
