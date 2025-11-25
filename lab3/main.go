package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Point - точка с целочисленными координатами (пиксель)
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// DrawRequest - запрос от фронтенда
type DrawRequest struct {
	Algorithm string `json:"algorithm"` // "step", "dda", "bresenham_line", "bresenham_circle"
	X1        int    `json:"x1"`
	Y1        int    `json:"y1"`
	X2        int    `json:"x2"` // Для линий
	Y2        int    `json:"y2"` // Для линий
	R         int    `json:"r"`  // Для окружности
}

// DrawResponse - ответ с точками и временем
type DrawResponse struct {
	Points []Point `json:"points"`
	TimeNs int64   `json:"time_ns"` // Время в наносекундах
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/api/draw", drawHandler)

	port := 8080
	log.Printf("Server running at http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func drawHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	start := time.Now()
	var points []Point

	switch req.Algorithm {
	case "step":
		points = stepByStep(req.X1, req.Y1, req.X2, req.Y2)
	case "dda":
		points = dda(req.X1, req.Y1, req.X2, req.Y2)
	case "bresenham_line":
		points = bresenhamLine(req.X1, req.Y1, req.X2, req.Y2)
	case "bresenham_circle":
		points = bresenhamCircle(req.X1, req.Y1, req.R)
	default:
		http.Error(w, "Unknown algorithm", http.StatusBadRequest)
		return
	}

	duration := time.Since(start).Nanoseconds()

	resp := DrawResponse{
		Points: points,
		TimeNs: duration,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- 1. Пошаговый алгоритм (y = kx + b) ---
// Основан на прямом уравнении прямой.
func stepByStep(x1, y1, x2, y2 int) []Point {
	var points []Point
	
	dx := x2 - x1
	dy := y2 - y1

	// Если линия вертикальная
	if dx == 0 {
		startY, endY := y1, y2
		if startY > endY {
			startY, endY = endY, startY
		}
		for y := startY; y <= endY; y++ {
			points = append(points, Point{X: x1, Y: y})
		}
		return points
	}

	k := float64(dy) / float64(dx)
	b := float64(y1) - k*float64(x1)

	if math.Abs(float64(dx)) >= math.Abs(float64(dy)) {
		// Идем по X
		step := 1
		if x2 < x1 { step = -1 }
		for x := x1; x != x2+step; x += step {
			y := int(math.Round(k*float64(x) + b))
			points = append(points, Point{X: x, Y: y})
		}
	} else {
		// Идем по Y (если наклон крутой)
		step := 1
		if y2 < y1 { step = -1 }
		for y := y1; y != y2+step; y += step {
			x := int(math.Round((float64(y) - b) / k))
			points = append(points, Point{X: x, Y: y})
		}
	}
	return points
}

// --- 2. Алгоритм ЦДА (DDA - Digital Differential Analyzer) ---
// Использование приращений dx и dy.
func dda(x1, y1, x2, y2 int) []Point {
	var points []Point
	
	dx := x2 - x1
	dy := y2 - y1
	
	steps := 0.0
	if math.Abs(float64(dx)) > math.Abs(float64(dy)) {
		steps = math.Abs(float64(dx))
	} else {
		steps = math.Abs(float64(dy))
	}
	
	xInc := float64(dx) / steps
	yInc := float64(dy) / steps
	
	x := float64(x1)
	y := float64(y1)
	
	for i := 0; i <= int(steps); i++ {
		points = append(points, Point{X: int(math.Round(x)), Y: int(math.Round(y))})
		x += xInc
		y += yInc
	}
	return points
}

// --- 3. Алгоритм Брезенхема (Отрезок) ---
// Только целочисленная арифметика.
func bresenhamLine(x1, y1, x2, y2 int) []Point {
	var points []Point
	
	dx := int(math.Abs(float64(x2 - x1)))
	dy := int(math.Abs(float64(y2 - y1)))
	
	sx := 1
	if x1 > x2 { sx = -1 }
	sy := 1
	if y1 > y2 { sy = -1 }
	
	err := dx - dy
	
	for {
		points = append(points, Point{X: x1, Y: y1})
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
	return points
}

// --- 4. Алгоритм Брезенхема (Окружность) ---
// Генерирует 1/8 часть и отражает симметрично.
func bresenhamCircle(xc, yc, r int) []Point {
	var points []Point
	
	x := 0
	y := r
	d := 3 - 2*r
	
	addPoints := func(xc, yc, x, y int) {
		points = append(points, 
			Point{xc + x, yc + y}, Point{xc - x, yc + y},
			Point{xc + x, yc - y}, Point{xc - x, yc - y},
			Point{xc + y, yc + x}, Point{xc - y, yc + x},
			Point{xc + y, yc - x}, Point{xc - y, yc - x},
		)
	}
	
	addPoints(xc, yc, x, y)
	
	for y >= x {
		x++
		if d > 0 {
			y--
			d = d + 4*(x-y) + 10
		} else {
			d = d + 4*x + 6
		}
		addPoints(xc, yc, x, y)
	}
	return points
}