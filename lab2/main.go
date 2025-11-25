package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"strconv"
)

type Response struct {
	ImageBase64 string `json:"image"` // Картинка для отображения
	Info        string `json:"info"`  // Текст с результатами (например, коэфф. сжатия)
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/api/process", processHandler)

	port := ":8080"
	log.Printf("Server starting at http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusBadRequest)
		return
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "Invalid image format", http.StatusBadRequest)
		return
	}

	method := r.FormValue("method")
	thresholdVal, _ := strconv.Atoi(r.FormValue("threshold_value"))

	var resImg image.Image
	infoText := ""

	switch method {
	case "contrast":
		// Вариант (Столбец): Линейное контрастирование
		resImg = linearContrastStretching(srcImg)
		infoText = "Применено линейное растяжение гистограммы."

	case "threshold_manual":
		// Вариант (Строка): Ручной порог
		gray := toGrayscale(srcImg)
		resImg = applyThreshold(gray, uint8(thresholdVal))
		infoText = fmt.Sprintf("Применен порог: %d", thresholdVal)

	case "threshold_otsu":
		// Вариант (Строка): Метод Оцу
		gray := toGrayscale(srcImg)
		t := calculateOtsuThreshold(gray)
		resImg = applyThreshold(gray, t)
		infoText = fmt.Sprintf("Рассчитанный порог Оцу: %d", t)

	case "compression_rle":
		// Лекция: Алгоритм RLE (сжатие без потерь)
		// Для RLE переводим в оттенки серого, чтобы сжимать байты яркости
		gray := toGrayscale(srcImg)
		resImg = gray // Возвращаем то же изображение (визуально оно не меняется при RLE)
		
		origSize, compSize, ratio := calculateRLEStats(gray)
		
		infoText = fmt.Sprintf("Результаты RLE сжатия:\nИсходный размер (пиксели): %d байт\nСжатый размер: %d байт\nКоэффициент сжатия: %.2f", 
			origSize, compSize, ratio)
		
		if ratio > 1.0 {
			infoText += "\n(Сжатие эффективно)"
		} else {
			infoText += "\n(Сжатие неэффективно - файл увеличился)"
		}

	default:
		http.Error(w, "Unknown method", http.StatusBadRequest)
		return
	}

	// 2. Кодируем результат в PNG -> Base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, resImg); err != nil {
		http.Error(w, "Failed to encode result", http.StatusInternalServerError)
		return
	}
	encodedStr := base64.StdEncoding.EncodeToString(buf.Bytes())

	// 3. Отправляем JSON
	resp := Response{
		ImageBase64: "data:image/png;base64," + encodedStr,
		Info:        infoText,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


func linearContrastStretching(img image.Image) image.Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	var minR, maxR, minG, maxG, minB, maxB uint8 = 255, 0, 255, 0, 255, 0

	// 1. Поиск min/max
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := rgba.RGBAAt(x, y)
			if c.R < minR { minR = c.R }; if c.R > maxR { maxR = c.R }
			if c.G < minG { minG = c.G }; if c.G > maxG { maxG = c.G }
			if c.B < minB { minB = c.B }; if c.B > maxB { maxB = c.B }
		}
	}

	stretch := func(val, min, max uint8) uint8 {
		if max == min { return val }
		return uint8((float64(val-min) / float64(max-min)) * 255)
	}

	// 2. Применение
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := rgba.RGBAAt(x, y)
			c.R = stretch(c.R, minR, maxR)
			c.G = stretch(c.G, minG, maxG)
			c.B = stretch(c.B, minB, maxB)
			rgba.SetRGBA(x, y, c)
		}
	}
	return rgba
}

func applyThreshold(img *image.Gray, t uint8) *image.Gray {
	bounds := img.Bounds()
	res := image.NewGray(bounds)
	for i := 0; i < len(img.Pix); i++ {
		if img.Pix[i] >= t {
			res.Pix[i] = 255
		} else {
			res.Pix[i] = 0
		}
	}
	return res
}

func calculateOtsuThreshold(img *image.Gray) uint8 {
	hist := make([]int, 256)
	for _, p := range img.Pix {
		hist[p]++
	}

	total := len(img.Pix)
	sum := 0.0
	for i := 0; i < 256; i++ { sum += float64(i * hist[i]) }

	sumB, wB, wF := 0.0, 0, 0
	maxVar, threshold := 0.0, 0

	for t := 0; t < 256; t++ {
		wB += hist[t]
		if wB == 0 { continue }
		wF = total - wB
		if wF == 0 { break }

		sumB += float64(t * hist[t])
		mB := sumB / float64(wB)
		mF := (sum - sumB) / float64(wF)

		v := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if v > maxVar {
			maxVar = v
			threshold = t
		}
	}
	return uint8(threshold)
}

// calculateRLEStats имитирует сжатие RLE для 8-битного канала
// Формат сжатия: [Значение][Количество]
// Например: 5 пикселей цвета 255 -> [255, 5] (2 байта вместо 5)
func calculateRLEStats(img *image.Gray) (originalSize, compressedSize int, ratio float64) {
	pixels := img.Pix
	originalSize = len(pixels)
	compressedSize = 0

	if originalSize == 0 {
		return 0, 0, 0
	}

	// Проход по байтам
	for i := 0; i < len(pixels); i++ {
		count := 1
		// Считаем длину серии одинаковых пикселей (максимум 255, т.к. count хранится в 1 байте)
		for i+1 < len(pixels) && pixels[i] == pixels[i+1] && count < 255 {
			i++
			count++
		}
		// Записываем пару [Значение, Счетчик] -> 2 байта
		compressedSize += 2
	}

	ratio = float64(originalSize) / float64(compressedSize)
	return
}

func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	draw.Draw(gray, bounds, img, bounds.Min, draw.Src)
	return gray
}