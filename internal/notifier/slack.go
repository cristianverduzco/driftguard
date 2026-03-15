package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackNotifier sends drift alerts to a Slack webhook
type SlackNotifier struct {
	webhookURL string
	enabled    bool
}

// DriftEvent represents a drift event to notify about
type DriftEvent struct {
	Kind      string
	Name      string
	Namespace string
	Reason    string
	Commit    string
}

// slackPayload is the Slack webhook request body
type slackPayload struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Title  string       `json:"title"`
	Fields []slackField `json:"fields"`
	Footer string       `json:"footer"`
	Ts     int64        `json:"ts"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new SlackNotifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		enabled:    webhookURL != "",
	}
}

// NotifyDrift sends a Slack message for a list of drifted resources
func (s *SlackNotifier) NotifyDrift(events []DriftEvent, commit string) error {
	if !s.enabled || len(events) == 0 {
		return nil
	}

	fields := []slackField{}
	for _, e := range events {
		fields = append(fields, slackField{
			Title: fmt.Sprintf("%s/%s", e.Kind, e.Name),
			Value: fmt.Sprintf("Namespace: `%s`\nReason: `%s`", e.Namespace, e.Reason),
			Short: true,
		})
	}

	payload := slackPayload{
		Text: fmt.Sprintf("⚠️ *DriftGuard detected %d drifted resource(s)*", len(events)),
		Attachments: []slackAttachment{
			{
				Color:  "#ff0000",
				Title:  "Drift Detected",
				Fields: fields,
				Footer: fmt.Sprintf("Commit: %s | DriftGuard", commit),
				Ts:     time.Now().Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := http.Post(s.webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned non-200 status: %d", resp.StatusCode)
	}

	fmt.Printf("📣 Slack notification sent for %d drifted resource(s)\n", len(events))
	return nil
}

// NotifyResolved sends a Slack message when drift is resolved
func (s *SlackNotifier) NotifyResolved(commit string) error {
	if !s.enabled {
		return nil
	}

	payload := slackPayload{
		Text: "✅ *DriftGuard: All resources are back in sync*",
		Attachments: []slackAttachment{
			{
				Color:  "#36a64f",
				Title:  "Drift Resolved",
				Fields: []slackField{},
				Footer: fmt.Sprintf("Commit: %s | DriftGuard", commit),
				Ts:     time.Now().Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := http.Post(s.webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("📣 Slack resolved notification sent")
	return nil
}