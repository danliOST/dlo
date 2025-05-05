package core

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

func NettruyenDownload(url_web string) {
	// Channels for chapter links and image download tasks
	chapterLinks := make(chan string)
	imageTasks := make(chan struct {
		url      string
		filePath string
	}, 100) // Buffered channel to limit concurrent downloads

	var wg sync.WaitGroup
	var mu sync.Mutex // Mutex for safe folder creation

	// Initialize collectors
	chapterColly := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36"),
		colly.MaxDepth(1),
	)
	imgColly := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36"),
		colly.MaxDepth(1),
	)

	// Main folder name
	var folderName string
	chapterColly.OnHTML("h1.title-detail", func(e *colly.HTMLElement) {
		mu.Lock()
		folderName = strings.TrimSpace(e.Text)
		folderName = strings.ReplaceAll(folderName, ":", "-") // Sanitize folder name
		mu.Unlock()
	})

	// Create main folder
	currentPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Không thể lấy đường dẫn hiện tại: %s\n", err)
		return
	}
	dirPath := filepath.Join(currentPath, folderName)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		fmt.Printf("Không thể tạo thư mục chính %s: %s\n", folderName, err)
		return
	}

	// Collect chapter links
	chapterColly.OnHTML("nav ul#desc a", func(e *colly.HTMLElement) {
		chapterURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapterLinks <- chapterURL
	})

	// Process image URLs
	imgColly.OnHTML("div.reading div.reading-detail.box_doc img.lozad", func(e *colly.HTMLElement) {
		imgURL := e.Attr("data-src")
		if imgURL == "" {
			fmt.Printf("Không tìm thấy URL hình ảnh\n")
			return
		}

		imgName := strings.Split(imgURL, "?")[0]
		imgName = path.Base(imgName)
		chapterName := strings.TrimSpace(e.Request.Ctx.Get("chapterName"))
		chapterPath := filepath.Join(dirPath, chapterName)

		mu.Lock()
		if err := os.MkdirAll(chapterPath, os.ModePerm); err != nil {
			fmt.Printf("Không thể tạo thư mục con %s: %s\n", chapterName, err)
			mu.Unlock()
			return
		}
		mu.Unlock()

		filePath := filepath.Join(chapterPath, imgName)
		imageTasks <- struct {
			url      string
			filePath string
		}{url: imgURL, filePath: filePath}
	})

	// Start image download workers
	const maxWorkers = 5
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 30 * time.Second,
			}
			for task := range imageTasks {
				req, _ := http.NewRequest("GET", task.url, nil)
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
				req.Header.Set("Referer", url_web)
				req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
				req.Header.Set("Accept-Encoding", "gzip, deflate, br")

				resp, err := client.Do(req)
				if err != nil {
					fmt.Printf("Lỗi khi tải hình ảnh %s: %s\n", task.url, err)
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					fmt.Printf("Lỗi HTTP %d khi tải %s\n", resp.StatusCode, task.url)
 2
                    continue
                }

                file, err := os.Create(task.filePath)
                if err != nil {
                    fmt.Printf("Không thể tạo tệp %s: %s\n", task.filePath, err)
                    continue
                }

                _, err = io.Copy(file, resp.Body)
                file.Close()
                if err != nil {
                    fmt.Printf("Lỗi khi lưu hình ảnh %s: %s\n", task.filePath, err)
                    continue
                }

                fmt.Printf("Đã tải %s vào %s\n", path.Base(task.filePath), task.filePath)
            }
        }()
    }

    // Start chapter processing
    go func() {
        for chapterURL := range chapterLinks {
            ctx := colly.NewContext()
            ctx.Put("chapterName", strings.TrimSpace(strings.Split(chapterURL, "/")[len(strings.Split(chapterURL, "/"))-1]))
            if err := imgColly.Request("GET", chapterURL, nil, ctx, nil); err != nil {
                fmt.Printf("Lỗi khi truy cập chương %s: %s\n", chapterURL, err)
            }
        }
        imgColly.Wait()
        close(imageTasks)
    }()

    // Start scraping
    fmt.Printf("Bắt đầu tải từ %s\n", url_web)
    if err := chapterColly.Visit(url_web); err != nil {
        fmt.Printf("Lỗi khi truy cập trang chính %s: %s\n", url_web, err)
        return
    }
    chapterColly.Wait()
    close(chapterLinks)

    // Wait for all downloads to complete
    wg.Wait()

    fmt.Printf("Đã tải xong tất cả chương vào %s\n", dirPath)
}