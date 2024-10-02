package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/xanzy/go-gitlab"
)

var (
	gitlabToken  = os.Getenv("GITLAB_TOKEN")
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	gitlabDomain = os.Getenv("GITLAB_DOMAIN") // Ensure this is set to your GitLab domain, e.g., https://gitlab.com
)

// GitLabEvent represents the structure for fetched GitLab events
type GitLabEvent struct {
	ID        int        `json:"id"`
	Action    string     `json:"action_name"`
	Target    string     `json:"target_title"`
	CreatedAt *time.Time `json:"created_at"`
}

// SummaryResponse represents the response structure from OpenAI API
type SummaryResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func main() {
	var (
		startDateStr string
		endDateStr   string
		gitlabUser   string
		eventsFile   string
		count        int
	)

	// Parse command-line flags
	flag.StringVar(&startDateStr, "start", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&endDateStr, "end", "", "End date (YYYY-MM-DD)")
	flag.StringVar(&gitlabUser, "user", "", "GitLab username")
	flag.StringVar(&eventsFile, "file", "", "Use a JSON file with user's event data as source instead of a Gitlab instance")
	flag.IntVar(&count, "count", 3, "Number of bullet points for summary")
	flag.Parse()

	if gitlabToken == "" || openaiAPIKey == "" || gitlabDomain == "" {
		log.Fatal("Please set GITLAB_DOMAIN, GITLAB_TOKEN and OPENAI_API_KEY environment variables")

	}

	if !strings.HasPrefix(gitlabDomain, "http") {
		log.Fatal("Environment variable GITLAB_DOMAIN must start with 'http'")
	}

	startDate, err := gitlab.ParseISOTime(startDateStr)
	if err != nil {
		log.Fatalf("Invalid start date: %v", err)
	}
	endDate, err := gitlab.ParseISOTime(endDateStr)
	if err != nil {
		log.Fatalf("Invalid end date: %v", err)
	}

	// Fetch events from GitLab using the SDK
	var events []*GitLabEvent

	if eventsFile != "" {
		fmt.Printf("Fetching user events data from file '%s'...\n", eventsFile)
		data, err := os.ReadFile(eventsFile)
		if err != nil {
			log.Fatalf("Failed to read file %s\n", eventsFile)
		}
		err = json.Unmarshal(data, &events)
		if err != nil {
			log.Fatalf("Failed to unmarshal JSON in file %s\n", eventsFile)
		}
	} else {
		fmt.Println("Fetching user events data from GitLab...")
		events, err = fetchGitLabEvents(startDate, endDate)
		if err != nil {
			log.Fatalf("Error fetching GitLab events: %v", err)
		}
		fmt.Println("Save events to JSON file...")
		fileName := fmt.Sprintf("gitlab_events.%s_%s.json", startDateStr, endDateStr)
		err = saveEventsToFile(events, fileName)
		if err != nil {
			log.Fatalf("Error saving events to file: %v", err)
		}
	}

	fmt.Printf("Summarizing %d event(s) using ChatGPT...\n", len(events))
	summary, err := summarizeEvents(events, count)
	if err != nil {
		log.Fatalf("Error summarizing events: %v", err)
	}

	// Output the summary
	fmt.Printf("\nGitlab Events for user %s:\n%s\n", gitlabUser, summary)
}

// // getUserIDByUsername fetches a user's ID based on their username
// func getUserIDByUsername(client *gitlab.Client, username string) (int, error) {
// 	users, _, err := client.Users.ListUsers(&gitlab.ListUsersOptions{
// 		Username: gitlab.String(username),
// 	})
// 	if err != nil {
// 		return 0, err
// 	}

// 	if len(users) == 0 {
// 		return 0, fmt.Errorf("user not found: %s", username)
// 	}

// 	return users[0].ID, nil
// }

// fetchGitLabEvents fetches user event data from GitLab API using the go-gitlab SDK
func fetchGitLabEvents(startDate, endDate gitlab.ISOTime) ([]*GitLabEvent, error) {
	client, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(gitlabDomain+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	// // Step 1: Get the user's ID by username
	// userID, err := getUserIDByUsername(client, username)
	// if err != nil {
	// 	return nil, fmt.Errorf("error getting user ID: %v", err)
	// }

	events := []*GitLabEvent{}
	opts := &gitlab.ListContributionEventsOptions{
		After:  &startDate,
		Before: &endDate,
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	for {
		userEvents, resp, err := client.Events.ListCurrentUserContributionEvents(opts)
		if err != nil {
			return nil, err
		}

		for _, e := range userEvents {
			events = append(events, &GitLabEvent{
				ID:        e.ID,
				Action:    e.ActionName,
				Target:    e.TargetTitle,
				CreatedAt: e.CreatedAt,
			})
		}

		if resp.CurrentPage >= resp.TotalPages {
			break
		}

		opts.Page = resp.NextPage
	}

	return events, nil
}

// saveEventsToFile saves GitLab event data to a JSON file
func saveEventsToFile(events []*GitLabEvent, filename string) error {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// summarizeEvents sends the event data to OpenAI API for summarization
func summarizeEvents(events []*GitLabEvent, count int) (string, error) {
	client := resty.New()

	// Prepare prompt for ChatGPT
	eventData, err := json.Marshal(events)
	if err != nil {
		return "", err
	}
	prompt := fmt.Sprintf(`Summarize the following GitLab user's events in %d
bullet points mostly of about ten words or less in terse, abbreviated human-sounding tone.

No periods at the end of each line, each bullet point starts with a past-tense verb, e.g. 'Pushed
bug fixes for... Engaged in discussions about... etc'.

Include the number of Merge Requests, Epics and Issues worked, opened, closed,
and version numbers upgraded to, the nature of code changes, pipeline runs. Do
not mention branch or MR deletions.

Include small inconsistencies in capitalization, punctuation or abbreviations to
make it look like a person wrote it, each bullet point should have varied
structure, some longer ones with commas, some shorter ones without:\n%s`, count, string(eventData))

	// Call OpenAI API
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+openaiAPIKey).
		SetBody(map[string]interface{}{
			// If this model doesn't work, use getOpenaiModels()
			"model":       "gpt-4o-2024-08-06",
			"messages":    []map[string]string{{"role": "system", "content": prompt}},
			"max_tokens":  150,
			"temperature": 0.7,
		}).
		SetResult(&SummaryResponse{}).
		Post("https://api.openai.com/v1/chat/completions")

	if err != nil {
		return "", err
	}

	summaryResp := resp.Result().(*SummaryResponse)

	if summaryResp.Error.Message != "" {
		return summaryResp.Error.Message, err
	}

	if len(summaryResp.Choices) == 0 {
		return "", errors.New(fmt.Sprintf("%v", resp))
	}
	return summaryResp.Choices[0].Message.Content, nil
}

type listModelsResult struct {
	Object string `json:"object"`
	Data   []struct {
		Id string `json:"id"`
	} `json:"data"`
}

func getOpenaiModels() (*listModelsResult, error) {
	client := resty.New()

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+openaiAPIKey).
		SetResult(&listModelsResult{}).
		Get("https://api.openai.com/v1/models")

	if err != nil {
		return nil, err
	}

	return resp.Result().(*listModelsResult), nil
}
