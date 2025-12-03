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
	Alpha float64 `json:"alpha"`
}

// DrawRequest - запрос от фронтенда
type DrawRequest struct {
	Algorithm string `json:"algorithm"`
	X1        int    `json:"x1"` // Начало
	Y1        int    `json:"y1"`
	X2        int    `json:"x2"` // Контрольная точка 1
	Y2        int    `json:"y2"`
	X3        int    `json:"x3"` // Контрольная точка 2 (для кривых)
	Y3        int    `json:"y3"`
	X4        int    `json:"x4"` // Конец (для кривых)
	Y4        int    `json:"y4"`
	R         int    `json:"r"`  // Радиус
}

// DrawResponse - ответ с точками и временем
type DrawResponse struct {
	Points []Point `json:"points"`
	TimeNs int64   `json:"time_ns"` // Время в наносекундах
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/api/draw", drawHandler)

	port := 8083
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
	case "casteljau":
		points = deCasteljau(req.X1, req.Y1, req.X2, req.Y2, req.X3, req.Y3, req.X4, req.Y4)
	case "wu":
		points = wuLine(req.X1, req.Y1, req.X2, req.Y2)
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
			Point{X: xc + x, Y: yc + y}, Point{X: xc - x, Y: yc + y},
			Point{X: xc + x, Y: yc - y}, Point{X: xc - x, Y: yc - y},
			Point{X: xc + y, Y: yc + x}, Point{X: xc - y, Y: yc + x},
			Point{X: xc + y, Y: yc - x}, Point{X: xc - y, Y: yc - x},
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


// --- 5. Алгоритм де Кастельжо (Кривая Безье) ---
// Строит кубическую кривую по 4 точкам.
func deCasteljau(x1, y1, x2, y2, x3, y3, x4, y4 int) []Point {
	var points []Point

	step := 0.005 

	for t := 0.0; t <= 1.0; t += step {

		q0x := float64(x1) + (float64(x2)-float64(x1))*t
		q0y := float64(y1) + (float64(y2)-float64(y1))*t

		q1x := float64(x2) + (float64(x3)-float64(x2))*t
		q1y := float64(y2) + (float64(y3)-float64(y2))*t

		q2x := float64(x3) + (float64(x4)-float64(x3))*t
		q2y := float64(y3) + (float64(y4)-float64(y3))*t

		r0x := q0x + (q1x-q0x)*t
		r0y := q0y + (q1y-q0y)*t

		r1x := q1x + (q2x-q1x)*t
		r1y := q1y + (q2y-q1y)*t

		bx := r0x + (r1x-r0x)*t
		by := r0y + (r1y-r0y)*t

		points = append(points, Point{X: int(math.Round(bx)), Y: int(math.Round(by))})
	}

	return points
}

// --- 6. Алгоритм Ву (Сглаживание) ---
func wuLine(x1, y1, x2, y2 int) []Point {
	var points []Point

	// Функция для добавления точки с яркостью
	plot := func(x, y int, c float64) {
		points = append(points, Point{X: x, Y: y, Alpha: c})
	}

	// Вспомогательные функции
	abs := func(x float64) float64 { return math.Abs(x) }
	ipart := func(x float64) int { return int(math.Floor(x)) }       // Целая часть
	fpart := func(x float64) float64 { return x - math.Floor(x) }    // Дробная часть
	rfpart := func(x float64) float64 { return 1.0 - fpart(x) }      // 1 - дробная

	// Проверяем крутизну
	steep := abs(float64(y2-y1)) > abs(float64(x2-x1))
	
	if steep {
		x1, y1 = y1, x1
		x2, y2 = y2, x2
	}
	if x2 < x1 {
		x1, x2 = x2, x1
		y1, y2 = y2, y1
	}

	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	gradient := dy / dx
	if dx == 0.0 {
		gradient = 1.0
	}

	// Обработка начальной точки
	xEnd := round(float64(x1))
	yEnd := float64(y1) + gradient*(float64(xEnd)-float64(x1))
	xGap := rfpart(float64(x1) + 0.5)
	
	xPixel1 := xEnd 
	yPixel1 := ipart(yEnd)
	
	if steep {
		plot(yPixel1, xPixel1, rfpart(yEnd)*xGap)
		plot(yPixel1+1, xPixel1, fpart(yEnd)*xGap)
	} else {
		plot(xPixel1, yPixel1, rfpart(yEnd)*xGap)
		plot(xPixel1, yPixel1+1, fpart(yEnd)*xGap)
	}
	intery := yEnd + gradient 

	// Обработка конечной точки
	xEnd2 := round(float64(x2))
	yEnd2 := float64(y2) + gradient*(float64(xEnd2)-float64(x2))
	xGap2 := fpart(float64(x2) + 0.5)
	
	xPixel2 := xEnd2 
	yPixel2 := ipart(yEnd2)

	if steep {
		plot(yPixel2, xPixel2, rfpart(yEnd2)*xGap2)
		plot(yPixel2+1, xPixel2, fpart(yEnd2)*xGap2)
	} else {
		plot(xPixel2, yPixel2, rfpart(yEnd2)*xGap2)
		plot(xPixel2, yPixel2+1, fpart(yEnd2)*xGap2)
	}

	// Основной цикл
	if steep {
		for x := xPixel1 + 1; x < xPixel2; x++ {
			plot(ipart(intery), x, rfpart(intery))
			plot(ipart(intery)+1, x, fpart(intery))
			intery += gradient
		}
	} else {
		for x := xPixel1 + 1; x < xPixel2; x++ {
			plot(x, ipart(intery), rfpart(intery))
			plot(x, ipart(intery)+1, fpart(intery))
			intery += gradient
		}
	}

	return points
}

func round(x float64) int {
	return int(math.Floor(x + 0.5))
}