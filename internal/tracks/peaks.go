package tracks

import (
	"encoding/json"
	"math"
)

// encodePeaks — нормализованные значения 0..1 кодируем в JSON массив
// округлённых int (0..255) для компактности (≤480B при 120 пиков).
// Decoded → float32 0..1 для удобства фронта.
func encodePeaks(peaks []float32) ([]byte, error) {
	out := make([]int, len(peaks))
	for i, p := range peaks {
		v := p
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		out[i] = int(math.Round(float64(v) * 255))
	}
	return json.Marshal(out)
}

// decodePeaks — обратно в []float32 0..1 для DTO.
// nil/empty/невалидный JSON → nil (без ошибки), фронт упадёт на свой
// generateWaveform fallback.
func decodePeaks(raw []byte) []float32 {
	if len(raw) == 0 {
		return nil
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err != nil {
		return nil
	}
	out := make([]float32, len(ints))
	for i, v := range ints {
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		out[i] = float32(v) / 255.0
	}
	return out
}
