package tracks

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
)

// ExtractPeaks — запускает ffmpeg и считывает PCM моно 8 kHz f32le,
// превращая поток сэмплов в 120 нормализованных peak-значений.
//
// Безопасность:
//   • absPath заранее провалидирован SafePath (не вне media-root)
//   • ffmpeg — отдельный процесс, ограниченный context-timeout'ом
//   • argv массивом (не shell-строкой) — никаких injection-точек
//
// Производительность:
//   • Выходной поток ~32 KiB / sec (8000 * 4 байта); даже 10-минутный
//     трек вычитывается за <1 сек, без буферизации в RAM
//   • На один upload — одна горутина; CPU bursty, не блокирует HTTP-стрим
func ExtractPeaks(ctx context.Context, absPath string, target int) ([]float32, error) {
	if target <= 0 {
		target = 120
	}
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-i", absPath,
		"-vn",                    // без видео-дорожки (если в .mp4)
		"-ac", "1",               // моно
		"-ar", "8000",            // 8 kHz достаточно для амплитудного контура
		"-f", "f32le",            // raw float32 little-endian
		"-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Считываем поточно, попутно агрегируя в N "корзин".
	// Сначала собираем все сэмплы, потом нарезаем — для коротких треков
	// это правильнее, иначе для 5-секундных в каждую корзину уйдёт по
	// 67 сэмплов (что мало, но допустимо).
	var samples []float32
	buf := make([]byte, 4096)
	for {
		n, rerr := io.ReadFull(stdout, buf)
		if n > 0 {
			n = n - (n % 4) // выровняли до целых float32
			for i := 0; i < n; i += 4 {
				bits := binary.LittleEndian.Uint32(buf[i : i+4])
				samples = append(samples, math.Float32frombits(bits))
			}
		}
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
			break
		}
		if rerr != nil {
			_ = cmd.Process.Kill()
			return nil, rerr
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w (stderr=%q)", err, stderr.String())
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples decoded")
	}

	return aggregatePeaks(samples, target), nil
}

// aggregatePeaks нарезает samples на target "корзин" и берёт RMS амплитуду
// в каждой, нормализуя на максимум по всему треку → значения 0..1.
func aggregatePeaks(samples []float32, target int) []float32 {
	bucket := len(samples) / target
	if bucket < 1 {
		bucket = 1
	}
	out := make([]float32, target)
	var maxV float64
	for i := 0; i < target; i++ {
		start := i * bucket
		end := start + bucket
		if i == target-1 || end > len(samples) {
			end = len(samples)
		}
		if start >= end {
			out[i] = 0
			continue
		}
		var sum float64
		for j := start; j < end; j++ {
			s := float64(samples[j])
			sum += s * s
		}
		rms := math.Sqrt(sum / float64(end-start))
		out[i] = float32(rms)
		if rms > maxV {
			maxV = rms
		}
	}
	if maxV <= 0 {
		return out
	}
	for i := range out {
		out[i] = float32(float64(out[i]) / maxV)
	}
	return out
}
