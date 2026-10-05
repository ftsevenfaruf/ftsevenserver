package botgen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

func GenerateUserAgents(number int) []string {
	rand.Seed(time.Now().UnixNano())

	var userAgents []string
	me := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	for i := 0; i < number; i++ {
		var sb strings.Builder
		sb.WriteString(me)
		for j := 0; j < 8; j++ {
			sb.WriteByte(byte(rand.Intn(36) + 48))
		}
		userAgents = append(userAgents, sb.String())
	}
	return userAgents
}

func GetViewerIds(videoID string, number int) (map[string]string, string, string) {
	fmt.Println("(+) Getting viewer ids with enhanced diagnostics...")

	userAgents := GenerateUserAgents(number)
	viewerIds := make(map[string]string)
	var channelName string
	var extractedVideoID string

	url := "https://rumble.com/embedJS/u3/?request=video&v=" + videoID

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Rate limiter to pace out requests slightly
	limiter := time.Tick(150 * time.Millisecond)

	for i, userAgent := range userAgents {
		wg.Add(1)
		go func(ua string, index int) {
			defer wg.Done()

			<-limiter

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				fmt.Printf("[%d] Request build error: %v\n", index, err)
				return
			}

			// Essential browser headers
			req.Header.Set("User-Agent", ua)
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			req.Header.Set("Sec-Fetch-Dest", "empty")
			req.Header.Set("Sec-Fetch-Mode", "cors")
			req.Header.Set("Sec-Fetch-Site", "same-origin")

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("[%d] Network error: %v\n", index, err)
				return
			}
			defer resp.Body.Close()

			// Print status code to reveal 403 blocks or challenges
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				fmt.Printf("[%d] Blocked! Status: %d | Response preview: %s\n", index, resp.StatusCode, string(bodyBytes[:min(len(bodyBytes), 100)]))
				return
			}

			var data map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				fmt.Printf("[%d] JSON decode error: %v\n", index, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()

			if vid, ok := data["vid"].(float64); ok {
				extractedVideoID = fmt.Sprintf("%.0f", vid)
			}

			if author, ok := data["author"].(map[string]interface{}); ok {
				if name, ok := author["name"].(string); ok {
					channelName = name
				}
			}

			viewerID, ok := data["viewer_id"].(string)
			if ok && viewerID != "" {
				viewerIds[viewerID] = ua
				fmt.Printf("(+) Successfully retrieved viewer ID #%d\n", len(viewerIds))
			}
		}(userAgent, i)
	}

	wg.Wait()
	fmt.Println("\n(+) Viewer IDs retrieval completed...")

	return viewerIds, extractedVideoID, channelName
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}