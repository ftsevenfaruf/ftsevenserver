package botgen

import (
    "encoding/json" // Added this
    "errors"
    "fmt"
    "net/http"
    "net/url"
    "regexp"
    "time"
)

func ExtractVideoID(RumbleURL string) (string, error) {
    fmt.Println("(+) Fetching ID via oEmbed API...")
    
    // 1. Clean the URL
    u, err := url.Parse(RumbleURL)
    if err != nil {
        return "", err
    }
    // Reconstruct URL without extra query params to be safe
    cleanURL := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)

    // 2. Use Rumble's oEmbed endpoint (much more reliable)
    apiURL := fmt.Sprintf("https://wn0.rumble.com/api/Media/oembed.json?url=%s", url.QueryEscape(cleanURL))

    client := &http.Client{Timeout: 10 * time.Second}
    req, err := http.NewRequest("GET", apiURL, nil)
    if err != nil {
        return "", err
    }

    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0 Safari/537.36")

    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("failed to reach API: %v", err)
    }
    defer resp.Body.Close()

    // 3. Decode the JSON response
    var result struct {
        HTML string `json:"html"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", errors.New("(-) Failed to parse API response. The link might be invalid or restricted")
    }

    // 4. Extract the ID from the <iframe> src inside the JSON
    // The ID is usually the alphanumeric string after /embed/
    re := regexp.MustCompile(`/embed/([a-zA-Z0-9]+)`)
    match := re.FindStringSubmatch(result.HTML)

    if len(match) < 2 {
        return "", errors.New("(-) Could not extract internal ID from API response")
    }

    videoID := match[1]
    fmt.Println("(+) Successfully retrieved internal ID:", videoID)
    return videoID, nil
}