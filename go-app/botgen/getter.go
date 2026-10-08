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

// GenerateUserAgents creates a slice of randomized user agents.
func GenerateUserAgents(number int) []string {
	rand.Seed(time.Now().UnixNano())

	var userAgents []string
	baseUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	
	for i := 0; i < number; i++ {
		var sb strings.Builder
		sb.WriteString(baseUA)
		// Add random suffix characters to simulate different browser builds/versions
		for j := 0; j < 8; j++ {
			sb.WriteByte(byte(rand.Intn(36) + 48)) // Random alphanumeric chars
		}
		userAgents = append(userAgents, sb.String())
	}
	return userAgents
}

// GetViewerIds fetches viewer IDs from Rumble's embed API with enhanced diagnostics, retry logic, and rate limiting.
func GetViewerIds(videoID string, number int) (map[string]string, string, string) {
	fmt.Println("(+) Getting viewer ids with enhanced diagnostics...")

	userAgents := GenerateUserAgents(number)
	viewerIds := make(map[string]string)
	var channelName string
	var extractedVideoID string

	// Base URL for the API call
	apiURL := fmt.Sprintf("https://rumble.com/embedJS/u3/?request=video&v=%s", videoID)
	// Referer should point to the embed page itself
	refererURL := fmt.Sprintf("https://rumble.com/embed/v%s.html", videoID)

	// Optimized HTTP Client
	client := &http.Client{
		Timeout: 15 * time.Second, // Increased timeout slightly to allow for retries
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 50,   // Limit idle connections to avoid resource leaks
			MaxConnsPerHost:     50,   // Limit total concurrent connections per host
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent requests to a safer number (e.g., 10-20)
	semaphore := make(chan struct{}, 20) // Max 20 concurrent requests

	// Rate limiter with jitter
	limiter := time.NewTicker(200 * time.Millisecond)
	defer limiter.Stop()

	for i, userAgent := range userAgents {
		wg.Add(1)
		go func(ua string, index int) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Wait for rate limiter tick
			<-limiter.C

			// Add small jitter to avoid synchronized bursts
			jitter := time.Duration(rand.Intn(100)) * time.Millisecond
			time.Sleep(jitter)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				fmt.Printf("[%d] Request build error: %v\n", index, err)
				return
			}

			// Essential browser headers including Referer which is critical for bypassing 403
			req.Header.Set("User-Agent", ua)
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			req.Header.Set("Sec-Fetch-Dest", "empty")
			req.Header.Set("Sec-Fetch-Mode", "cors")
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("Referer", refererURL)
			req.Header.Set("Origin", "https://rumble.com")

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("[%d] Network error: %v\n", index, err)
				return
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("[%d] Body read error: %v\n", index, err)
				return
			}

			// Print status code and preview to reveal 403 blocks or challenges
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("[%d] Blocked! Status: %d | Response preview: %s\n", index, resp.StatusCode, string(bodyBytes[:min(len(bodyBytes), 100)]))
				return
			}

			var data map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				fmt.Printf("[%d] JSON decode error: %v | Raw: %s\n", index, err, string(bodyBytes[:min(len(bodyBytes), 200)]))
				return
			}

			mu.Lock()
			defer mu.Unlock()

			// Extract Video ID
			if vid, ok := data["vid"].(float64); ok {
				extractedVideoID = fmt.Sprintf("%.0f", vid)
			}

			// Extract Channel Name
			if author, ok := data["author"].(map[string]interface{}); ok {
				if name, ok := author["name"].(string); ok {
					channelName = name
				}
			}

			// Extract Viewer ID
			viewerID, ok := data["viewer_id"].(string)
			if ok && viewerID != "" {
				viewerIds[viewerID] = ua
				fmt.Printf("(+) Successfully retrieved viewer ID #%d: %s\n", len(viewerIds), viewerID)
			} else {
				fmt.Printf("[%d] No viewer_id found in response. Keys: %v\n", index, getKeys(data))
			}
		}(userAgent, i)
	}

	wg.Wait()
	fmt.Println("\n(+) Viewer IDs retrieval completed...")

	return viewerIds, extractedVideoID, channelName
}

// Helper function to get keys from a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}