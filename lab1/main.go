package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
)

type ConvertRequest struct {
	Model  string                 `json:"model"` // "rgb", "cmyk", "hsv"
	Values map[string]float64     `json:"values"`
}

type ConvertResponse struct {
	RGB  RGBModel  `json:"rgb"`
	CMYK CMYKModel `json:"cmyk"`
	HSV  HSVModel  `json:"hsv"`
}

type RGBModel struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

type CMYKModel struct {
	C float64 `json:"c"`
	M float64 `json:"m"`
	Y float64 `json:"y"`
	K float64 `json:"k"`
}

type HSVModel struct {
	H float64 `json:"h"`
	S float64 `json:"s"`
	V float64 `json:"v"`
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/api/convert", convertHandler)

	port := 8079
	log.Printf("Server running at http://localhost:%d/ (serving ./static)\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func convertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Определяем базовый цвет (RGB) на основе входной модели
	var rgb RGBModel
	
	switch req.Model {
	case "rgb":
		rf, okR := req.Values["r"]
		gf, okG := req.Values["g"]
		bf, okB := req.Values["b"]
		if !okR || !okG || !okB {
			http.Error(w, "rgb requires r,g,b", http.StatusBadRequest)
			return
		}
		rgb = RGBModel{
			R: clampInt(int(math.Round(rf)), 0, 255),
			G: clampInt(int(math.Round(gf)), 0, 255),
			B: clampInt(int(math.Round(bf)), 0, 255),
		}
	case "cmyk":
		cf, okC := req.Values["c"]
		mf, okM := req.Values["m"]
		yf, okY := req.Values["y"]
		kf, okK := req.Values["k"]
		if !okC || !okM || !okY || !okK {
			http.Error(w, "cmyk requires c,m,y,k", http.StatusBadRequest)
			return
		}
		r64, g64, b64 := CMYKToRGB(cf, mf, yf, kf)
		rgb = RGBModel{
			R: clampInt(int(math.Round(r64)), 0, 255),
			G: clampInt(int(math.Round(g64)), 0, 255),
			B: clampInt(int(math.Round(b64)), 0, 255),
		}
	case "hsv":
		hf, okH := req.Values["h"]
		sf, okS := req.Values["s"]
		vf, okV := req.Values["v"]
		if !okH || !okS || !okV {
			http.Error(w, "hsv requires h,s,v", http.StatusBadRequest)
			return
		}
		r64, g64, b64 := HSVToRGB(hf, sf, vf)
		rgb = RGBModel{
			R: clampInt(int(math.Round(r64)), 0, 255),
			G: clampInt(int(math.Round(g64)), 0, 255),
			B: clampInt(int(math.Round(b64)), 0, 255),
		}
	default:
		http.Error(w, "model must be one of: rgb, cmyk, hsv", http.StatusBadRequest)
		return
	}

	// 2. Рассчитываем значения для всех моделей из полученного RGB
	c, m, y, k := RGBToCMYK(rgb.R, rgb.G, rgb.B)
	h, s, v := RGBToHSV(rgb.R, rgb.G, rgb.B)

	// Формируем структуры ответа с расчетными данными
	respCMYK := CMYKModel{
		C: roundFloat(c*100, 2),
		M: roundFloat(m*100, 2),
		Y: roundFloat(y*100, 2),
		K: roundFloat(k*100, 2),
	}
	respHSV := HSVModel{
		H: roundFloat(h, 2),
		S: roundFloat(s*100, 2),
		V: roundFloat(v*100, 2),
	}

	// 3. ВАЖНО: Перезаписываем значения для текущей активной модели теми, что ввел пользователь.
	// Это предотвращает сброс ползунков из-за математических округлений или особенностей моделей
	// (например, при K=100 в CMYK значения C,M,Y математически не важны, но интерфейс их терять не должен).
	if req.Model == "cmyk" {
		respCMYK.C = req.Values["c"]
		respCMYK.M = req.Values["m"]
		respCMYK.Y = req.Values["y"]
		respCMYK.K = req.Values["k"]
	} else if req.Model == "hsv" {
		respHSV.H = req.Values["h"]
		respHSV.S = req.Values["s"]
		respHSV.V = req.Values["v"]
	}
	// Для RGB обычно полезнее оставить clamp-значения (0-255), поэтому их не перезаписываем.

	resp := ConvertResponse{
		RGB:  rgb,
		CMYK: respCMYK,
		HSV:  respHSV,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------- Конвертации ----------

// CMYK (0..100%) -> RGB (0..255)
func CMYKToRGB(cPct, mPct, yPct, kPct float64) (r, g, b float64) {
	c := clampFloat(cPct/100.0, 0, 1)
	m := clampFloat(mPct/100.0, 0, 1)
	y := clampFloat(yPct/100.0, 0, 1)
	k := clampFloat(kPct/100.0, 0, 1)

	// стандартый подход: r = 255*(1-c)*(1-k)
	r = (1.0-c)*(1.0-k) * 255.0
	g = (1.0-m)*(1.0-k) * 255.0
	b = (1.0-y)*(1.0-k) * 255.0
	return
}

// RGB 0..255 -> CMYK 0..1
func RGBToCMYK(rInt, gInt, bInt int) (c, m, y, k float64) {
	r := clampFloat(float64(rInt)/255.0, 0, 1)
	g := clampFloat(float64(gInt)/255.0, 0, 1)
	b := clampFloat(float64(bInt)/255.0, 0, 1)

	k = 1 - math.Max(r, math.Max(g, b))
	if almostEqual(k, 1.0) {
		// черный
		return 0, 0, 0, 1
	}
	c = (1 - r - k) / (1 - k)
	m = (1 - g - k) / (1 - k)
	y = (1 - b - k) / (1 - k)
	// защита от негативных нулей
	c = clampFloat(c, 0, 1)
	m = clampFloat(m, 0, 1)
	y = clampFloat(y, 0, 1)
	return
}

// RGB 0..255 -> HSV: H 0..360, S 0..1, V 0..1
func RGBToHSV(rInt, gInt, bInt int) (h, s, v float64) {
	r := clampFloat(float64(rInt)/255.0, 0, 1)
	g := clampFloat(float64(gInt)/255.0, 0, 1)
	b := clampFloat(float64(bInt)/255.0, 0, 1)

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	// Hue
	if almostEqual(delta, 0) {
		h = 0
	} else {
		switch {
		case almostEqual(max, r):
			h = 60 * math.Mod((g-b)/delta, 6)
		case almostEqual(max, g):
			h = 60 * (((b-r)/delta) + 2)
		default: // max == b
			h = 60 * (((r-g)/delta) + 4)
		}
		if h < 0 {
			h += 360
		}
	}

	// Saturation
	if almostEqual(max, 0) {
		s = 0
	} else {
		s = delta / max
	}

	v = max
	return
}

// HSV -> RGB (inputs: H 0..360, S 0..100, V 0..100) -> RGB 0..255
func HSVToRGB(hDeg, sPct, vPct float64) (r, g, b float64) {
	h := math.Mod(hDeg, 360)
	if h < 0 {
		h += 360
	}
	s := clampFloat(sPct/100.0, 0, 1)
	v := clampFloat(vPct/100.0, 0, 1)

	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := v - c

	var rp, gp, bp float64
	switch {
	case 0 <= h && h < 60:
		rp, gp, bp = c, x, 0
	case 60 <= h && h < 120:
		rp, gp, bp = x, c, 0
	case 120 <= h && h < 180:
		rp, gp, bp = 0, c, x
	case 180 <= h && h < 240:
		rp, gp, bp = 0, x, c
	case 240 <= h && h < 300:
		rp, gp, bp = x, 0, c
	default:
		rp, gp, bp = c, 0, x
	}

	r = (rp + m) * 255.0
	g = (gp + m) * 255.0
	b = (bp + m) * 255.0
	return
}


func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clampFloat(v, low, high float64) float64 {
	if math.IsNaN(v) {
		return low
	}
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func roundFloat(x float64, prec int) float64 {
	p := math.Pow(10, float64(prec))
	return math.Round(x*p) / p
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}


func init() {
	dir := "static"
	if abs, err := filepath.Abs(dir); err == nil {
		_ = abs
	}
}

