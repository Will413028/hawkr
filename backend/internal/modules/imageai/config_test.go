package imageai

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func TestReadConfigDefaultsAndRejectsLessThanOne(t *testing.T) {
	e := config.FromMap(nil)
	if c := ReadConfig(e); c.MaxConcurrent < 1 || e.Err() != nil {
		t.Fatalf("預設 = %+v err = %v", c, e.Err())
	}
	e = config.FromMap(map[string]string{"IMAGEAI_MAX_CONCURRENT": "7"})
	if c := ReadConfig(e); c.MaxConcurrent != 7 || e.Err() != nil {
		t.Fatalf("7 = %+v err = %v", c, e.Err())
	}
	for _, v := range []string{"0", "-3"} {
		e = config.FromMap(map[string]string{"IMAGEAI_MAX_CONCURRENT": v})
		ReadConfig(e)
		if e.Err() == nil {
			t.Errorf("IMAGEAI_MAX_CONCURRENT=%s 要被擋下", v)
		}
	}
}
