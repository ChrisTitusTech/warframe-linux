//go:build manual && linux

package memoryScanner

import (
	"os"
	"testing"
)

func TestReadMapsManual(t *testing.T) {
	regions, err := readMaps(os.Getpid())
	if err != nil {
		t.Fatalf("readMaps() error = %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("readMaps() returned no regions")
	}
	for i, region := range regions {
		if region.end <= region.start {
			t.Fatalf("region %d = %#v, want end > start", i, region)
		}
	}
}
