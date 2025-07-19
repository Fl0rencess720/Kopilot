package logsource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LokiClient struct {
	address string
}

type LokiResponse struct {
	Status string   `json:"status"`
	Data   LokiData `json:"data"`
}

type LokiData struct {
	ResultType string       `json:"resultType"`
	Result     []LokiResult `json:"result"`
}

type LokiResult struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type LokiQueryOptions struct {
	PodName   string
	Namespace string
	Filter    string
	Limit     int
	TimeRange time.Duration
}

func NewLokiClient(address string) *LokiClient {
	return &LokiClient{address: address}
}

func (l *LokiClient) FetchLogs(opts LokiQueryOptions) (string, error) {
	if opts.PodName == "" || opts.Namespace == "" {
		return "", fmt.Errorf("PodName and Namespace are required in options")
	}

	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.TimeRange <= 0 {
		opts.TimeRange = 700 * time.Hour
	}

	streamSelector := fmt.Sprintf(`{namespace="%s",pod="%s"}`, opts.Namespace, opts.PodName)
	finalQuery := streamSelector
	if opts.Filter != "" {
		finalQuery = fmt.Sprintf("%s %s", streamSelector, opts.Filter)
	}
	lokiURL, err := url.Parse(l.address)
	if err != nil {
		return "", fmt.Errorf("invalid loki address: %w", err)
	}
	lokiURL.Path = "/loki/api/v1/query_range"
	endTime := time.Now()
	startTime := endTime.Add(-opts.TimeRange)

	params := url.Values{}
	params.Add("query", finalQuery)
	params.Add("limit", fmt.Sprintf("%d", opts.Limit))
	params.Add("direction", "backward")
	params.Add("start", startTime.Format(time.RFC3339Nano))
	params.Add("end", endTime.Format(time.RFC3339Nano))

	lokiURL.RawQuery = params.Encode()
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", lokiURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loki returned non-200 status code: %d", resp.StatusCode)
	}

	var lokiResp LokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return "", fmt.Errorf("failed to decode Loki response: %w", err)
	}

	if lokiResp.Status != "success" {
		return "", fmt.Errorf("loki query failed with status: %s", lokiResp.Status)
	}

	var collectedLogs []string
	for _, result := range lokiResp.Data.Result {
		for _, valuePair := range result.Values {
			collectedLogs = append(collectedLogs, valuePair[1])
		}
	}

	for i, j := 0, len(collectedLogs)-1; i < j; i, j = i+1, j-1 {
		collectedLogs[i], collectedLogs[j] = collectedLogs[j], collectedLogs[i]
	}
	return strings.Join(collectedLogs, "\n"), nil
}
