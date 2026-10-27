package usecase

import (
	"testing"
	"time"
)

func TestMonthlyChunks(t *testing.T) {
	start := time.Date(2012, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2012, 3, 10, 0, 0, 0, 0, time.UTC)
	chunks := MonthlyChunks(start, end)
	if len(chunks) != 3 {
		t.Fatalf("= %d 個區間，預期 3 個（1月、2月、3月）", len(chunks))
	}
	if !chunks[0].Start.Equal(start) {
		t.Errorf("第一段起始應為 %v，得到 %v", start, chunks[0].Start)
	}
	if !chunks[2].End.Equal(end) {
		t.Errorf("最後一段結束應為 %v，得到 %v", end, chunks[2].End)
	}
	if chunks[0].End.After(chunks[1].Start) {
		t.Error("區間不得重疊")
	}
}

func TestMonthlyChunksSingleDay(t *testing.T) {
	d := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	chunks := MonthlyChunks(d, d)
	if len(chunks) != 1 || !chunks[0].Start.Equal(d) || !chunks[0].End.Equal(d) {
		t.Errorf("單日應回一段，得到 %+v", chunks)
	}
}
