package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func probeEndpoint(ctx context.Context, endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid endpoint URL: %q", endpoint)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("endpoint must use http or https")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "os-pangolin-healthcheck")
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("endpoint returned %s", strings.TrimSpace(resp.Status))
	}
	fmt.Printf("endpoint reachable: %s\n", resp.Status)
	return nil
}
