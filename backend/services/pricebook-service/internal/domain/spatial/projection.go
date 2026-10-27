// Package spatial ports Hawkr_Transformer_Pricebook@2b64d9b's
// dependency-light spatial engine (src/hawkr_price_ai/spatial/
// {interpolation,reference}.py) into Go, per the design in
// docs/superpowers/specs/2026-08-08-pricebook-spatial-surface-go-native-design.md.
// Every exported function here has a Python source doc comment; any
// numerical deviation from that source is a bug, not a stylistic choice —
// see testdata/parity for the cross-language golden-fixture harness.
package spatial

import (
	"fmt"
	"math"
)

const (
	grs80SemiMajorAxis     = 6_378_137.0
	grs80InverseFlattening = 298.257222101
	tm2Scale               = 0.9999
	tm2FalseEasting        = 250_000.0
	tm2FalseNorthing       = 0.0
)

// tm2CentralMeridianByEPSG mirrors interpolation.py's
// _TM2_CENTRAL_MERIDIAN_BY_EPSG: EPSG:3825 (Kinmen/Matsu, TM2 zone 119) and
// EPSG:3826 (Taiwan main island, TM2 zone 121) are the only two
// interpolation CRSes this system supports — the same two the mirrored
// county boundary data (geography.CountyRegion.InterpolationEPSG) uses.
var tm2CentralMeridianByEPSG = map[int]float64{
	3825: 119.0 * math.Pi / 180,
	3826: 121.0 * math.Pi / 180,
}

func centralMeridian(epsg int) (float64, error) {
	cm, ok := tm2CentralMeridianByEPSG[epsg]
	if !ok {
		return 0, fmt.Errorf("spatial: interpolation EPSG must be 3825 or 3826, got %d", epsg)
	}
	return cm, nil
}

// WGS84ToTWD97TM2 projects WGS84 lon/lat degrees to TWD97 TM2 meters,
// mirroring interpolation.py:189-233 (wgs84_to_twd97_tm2) line for line —
// GRS80 ellipsoid, Snyder's forward Transverse Mercator series.
func WGS84ToTWD97TM2(longitudeDeg, latitudeDeg float64, epsg int) (x, y float64, err error) {
	centralMeridian, err := centralMeridian(epsg)
	if err != nil {
		return 0, 0, err
	}
	lon := longitudeDeg * math.Pi / 180
	lat := latitudeDeg * math.Pi / 180
	flattening := 1 / grs80InverseFlattening
	eccSq := flattening * (2 - flattening)
	secondEccSq := eccSq / (1 - eccSq)
	sinLat := math.Sin(lat)
	cosLat := math.Cos(lat)
	tanLat := math.Tan(lat)
	n := grs80SemiMajorAxis / math.Sqrt(1-eccSq*sinLat*sinLat)
	t := tanLat * tanLat
	c := secondEccSq * cosLat * cosLat
	aTerm := (lon - centralMeridian) * cosLat
	e4 := eccSq * eccSq
	e6 := eccSq * eccSq * eccSq
	meridional := grs80SemiMajorAxis * ((1-eccSq/4-3*e4/64-5*e6/256)*lat -
		(3*eccSq/8+3*e4/32+45*e6/1024)*math.Sin(2*lat) +
		(15*e4/256+45*e6/1024)*math.Sin(4*lat) -
		(35*e6/3072)*math.Sin(6*lat))
	x = tm2FalseEasting + tm2Scale*n*(aTerm+
		(1-t+c)*(aTerm*aTerm*aTerm)/6+
		(5-18*t+t*t+72*c-58*secondEccSq)*(aTerm*aTerm*aTerm*aTerm*aTerm)/120)
	y = tm2FalseNorthing + tm2Scale*(meridional+
		n*tanLat*(aTerm*aTerm/2+
			(5-t+9*c+4*c*c)*(aTerm*aTerm*aTerm*aTerm)/24+
			(61-58*t+t*t+600*c-330*secondEccSq)*(aTerm*aTerm*aTerm*aTerm*aTerm*aTerm)/720))
	return x, y, nil
}

// TWD97TM2ToWGS84 is the inverse of WGS84ToTWD97TM2, mirroring
// interpolation.py:236-291 (twd97_tm2_to_wgs84) line for line — footpoint
// latitude iteration + Snyder's inverse Transverse Mercator series.
func TWD97TM2ToWGS84(x, y float64, epsg int) (longitudeDeg, latitudeDeg float64, err error) {
	centralMeridian, err := centralMeridian(epsg)
	if err != nil {
		return 0, 0, err
	}
	east := (x - tm2FalseEasting) / tm2Scale
	north := (y - tm2FalseNorthing) / tm2Scale
	flattening := 1 / grs80InverseFlattening
	eccSq := flattening * (2 - flattening)
	secondEccSq := eccSq / (1 - eccSq)
	e4 := eccSq * eccSq
	e6 := eccSq * eccSq * eccSq
	mu := north / (grs80SemiMajorAxis * (1 - eccSq/4 - 3*e4/64 - 5*e6/256))
	e1 := (1 - math.Sqrt(1-eccSq)) / (1 + math.Sqrt(1-eccSq))
	footpoint := mu +
		(3*e1/2-27*(e1*e1*e1)/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*(e1*e1*e1*e1)/32)*math.Sin(4*mu) +
		(151*(e1*e1*e1)/96)*math.Sin(6*mu) +
		(1097*(e1*e1*e1*e1)/512)*math.Sin(8*mu)
	sinFootpoint := math.Sin(footpoint)
	cosFootpoint := math.Cos(footpoint)
	tanFootpoint := math.Tan(footpoint)
	n1 := grs80SemiMajorAxis / math.Sqrt(1-eccSq*sinFootpoint*sinFootpoint)
	r1 := grs80SemiMajorAxis * (1 - eccSq) / math.Pow(1-eccSq*sinFootpoint*sinFootpoint, 1.5)
	t1 := tanFootpoint * tanFootpoint
	c1 := secondEccSq * cosFootpoint * cosFootpoint
	d := east / n1
	latitude := footpoint - (n1*tanFootpoint/r1)*(d*d/2-
		(5+3*t1+10*c1-4*c1*c1-9*secondEccSq)*(d*d*d*d)/24+
		(61+90*t1+298*c1+45*t1*t1-252*secondEccSq-3*c1*c1)*(d*d*d*d*d*d)/720)
	longitude := centralMeridian +
		(d-
			(1+2*t1+c1)*(d*d*d)/6+
			(5-2*c1+28*t1-3*c1*c1+8*secondEccSq+24*t1*t1)*(d*d*d*d*d)/120)/cosFootpoint
	return longitude * 180 / math.Pi, latitude * 180 / math.Pi, nil
}
